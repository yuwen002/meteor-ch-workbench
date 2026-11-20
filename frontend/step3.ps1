$ErrorActionPreference = 'Stop'
$enc = [System.Text.UTF8Encoding]::new($false)
$p = 'C:\works\go_works\meteor-ch-workbench\frontend\src\views\QueryEditor.vue'
$c = [System.IO.File]::ReadAllText($p, $enc)
$c = $c -replace "`r`n", "`n"
$L = $c.Split("`n")

function FindIdx($prefix, $from) {
  for ($i = $from; $i -lt $L.Count; $i++) {
    if ($L[$i].TrimStart().StartsWith($prefix)) { return $i }
  }
  return -1
}

# 1) 历史函数整体块：loadHistory 起，formatHistoryTime 函数结束（含其后的空行）
$s = FindIdx 'async function loadHistory() {' 0
$e = FindIdx 'function formatHistoryTime(ts: string): string {' 0
if ($s -lt 0 -or $e -lt 0) { throw "anchor fail $s $e" }
# 保留 toggleHistory 与 applyHistory
$tog = FindIdx 'async function toggleHistory() {' $s
$app = FindIdx 'function applyHistory(entry: HistoryEntry) {' $s
if ($tog -lt 0 -or $app -lt 0) { throw "anchor2 fail $tog $app" }

$keepToggle = @(
'function toggleHistory() {',
'  showHistory.value = !showHistory.value',
'}',
''
)
$keepApply = @(
'function applyHistory(sql: string) {',
'  if (!editor) return',
'  editor.setValue(sql)',
'  syncTabSQL()',
'  showHistory.value = false',
'}'
)

$newBlock = $keepToggle + $keepApply
$L = $L[0..($s - 1)] + $newBlock + $L[($e + 7)..($L.Count - 1)]

# 2) 删除状态两行
$i = FindIdx 'const historyEntries = ref<HistoryEntry[]>([])' 0
if ($i -lt 0) { throw 'state1 fail' }
$j = FindIdx 'const historyLoading = ref(false)' 0
if ($j -le $i) { throw 'state2 fail' }
$L = $L[0..($i - 1)] + $L[($j + 1)..($L.Count - 1)]

# 3) 导入
$i = FindIdx "import type { QueryResult, HistoryEntry } from '../types'" 0
if ($i -lt 0) { throw 'import fail' }
$L[$i] = "import type { QueryResult } from '../types'"
$j = FindIdx "import DatabaseTree from '../components/DatabaseTree.vue'" 0
if ($j -lt 0) { throw 'import2 fail' }
$L = $L[0..$j] + @("import HistoryPanel from '../components/HistoryPanel.vue'") + $L[($j + 1)..($L.Count - 1)]

# 4) 统一 CRLF 写回
[System.IO.File]::WriteAllText($p, (($L -join "`r`n")), $enc)
Write-Output ("lines=" + $L.Count)