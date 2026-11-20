import { ref, nextTick, onBeforeUnmount, type Ref } from 'vue'
import type * as Monaco from 'monaco-editor/editor/editor.api.js'
import { createSqlCompletion, type SqlCompletionHandle } from './useSqlCompletion'

export interface UseMonacoOptions {
  // 编辑器内容变化（含程序化 setValue）时的回调，用于同步当前标签 SQL
  onContentChange: () => void
  // 光标 / 选区变化时回调，用于同步可用 SQL
  onSelectionChange: () => void
  // Ctrl+Enter 触发
  onRun: () => void
  // 补全所需的当前活动连接 ID（无连接时返回 null）
  getConnectionId?: () => string | null
}

export interface UseMonacoReturn {
  editorRef: Ref<HTMLElement | null>
  editor: Monaco.editor.IStandaloneCodeEditor | null
  initEditor: (initialValue: string) => Promise<void>
  getEditor: () => Monaco.editor.IStandaloneCodeEditor | null
  disposeEditor: () => void
  invalidateCompletion: (connectionId?: string) => void
}

// 空 Blob Worker：SQL 语言不使用 worker，仅需满足 Monaco 的 Worker 创建契约
const WORKER_CODE = `
  self.languages = self.languages || {};
  self.languages['sql'] = { mimetypes: ['text/x-sql', 'text/x-mysql'] };
  self.onmessage = function(e) {
    var msg = e.data;
    if (msg && msg.type === 'initialize') {
      self.postMessage({ type: 'initialized' });
    }
  };
`

function installMonacoEnvironment() {
  const blob = new Blob([WORKER_CODE], { type: 'application/javascript' })
  ;(self as any).MonacoEnvironment = {
    getWorkerUrl(_moduleId: string, _label: string) {
      return URL.createObjectURL(blob)
    },
  }
}

function defineTheme(monaco: typeof Monaco) {
  monaco.editor.defineTheme('ch-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [],
    colors: {
      'editor.background': '#1d1e22',
      'editor.foreground': '#e4e4e7',
      'editor.lineHighlightBackground': '#2a2d33',
      'editorCursor.foreground': '#e74c3c',
      'editor.selectionBackground': 'rgba(231, 76, 60, 0.3)',
      'editorLineNumber.foreground': '#5c6270',
      'editorLineNumber.activeForeground': '#e4e4e7',
    },
  })
  monaco.editor.setTheme('ch-dark')
}

export function useMonaco(options: UseMonacoOptions): UseMonacoReturn {
  const editorRef = ref<HTMLElement | null>(null)
  let editor: Monaco.editor.IStandaloneCodeEditor | null = null
  let completion: SqlCompletionHandle | null = null

  async function initEditor(initialValue: string) {
    await nextTick()
    if (!editorRef.value || editor) return

    installMonacoEnvironment()

    const [monacoApi] = await Promise.all([
      import('monaco-editor/editor/editor.api.js'),
      import('monaco-editor/languages/definitions/sql/register.js'),
    ])
    const monaco = monacoApi as unknown as typeof Monaco
    if (!editorRef.value) return

    editor = monaco.editor.create(editorRef.value, {
      value: initialValue,
      language: 'sql',
      theme: 'vs-dark',
      automaticLayout: true,
      minimap: { enabled: false },
      fontSize: 13,
      fontFamily: 'Consolas, "JetBrains Mono", Menlo, monospace',
      tabSize: 2,
      wordWrap: 'on',
      scrollBeyondLastLine: false,
      lineNumbers: 'on',
      renderLineHighlight: 'line',
      cursorBlinking: 'smooth',
      smoothScrolling: true,
    })

    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
      options.onRun()
    })

    editor.onDidChangeModelContent(() => {
      options.onContentChange()
    })

    editor.onDidChangeCursorSelection(() => {
      options.onSelectionChange()
    })

    completion = createSqlCompletion(monaco, {
      getConnectionId: () => options.getConnectionId?.() ?? null,
      getText: () => editor?.getModel()?.getValue() ?? '',
    })
    completion.register()

    defineTheme(monaco)
  }

  function getEditor() {
    return editor
  }

  function invalidateCompletion(connectionId?: string) {
    completion?.invalidate(connectionId)
  }

  function disposeEditor() {
    completion?.dispose()
    completion = null
    editor?.dispose()
    editor = null
  }

  onBeforeUnmount(() => {
    disposeEditor()
  })

  return { editorRef, editor, initEditor, getEditor, disposeEditor, invalidateCompletion }
}