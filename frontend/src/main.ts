import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useAppStore } from './stores/app'
import './style.css'

const app = createApp(App)

app.config.errorHandler = (err, _instance, info) => {
  const text = '[vue:error] ' + info + '\n' + (err instanceof Error ? err.stack || err.message : String(err))
  console.error(text)
  let el = document.getElementById('__err')
  if (!el) {
    el = document.createElement('pre')
    el.id = '__err'
    el.style.cssText =
      'position:fixed;top:0;left:0;right:0;z-index:99999;max-height:40vh;overflow:auto;margin:0;padding:10px;background:#7f1d1d;color:#fff;font:12px/1.5 Consolas,monospace;white-space:pre-wrap;word-break:break-all;border-bottom:2px solid #ef4444;'
    document.body.appendChild(el)
  }
  el.textContent += text + '\n'
}

app.use(createPinia())
app.use(router)
app.use(i18n)

app.directive('focus', {
  mounted(el: HTMLElement) {
    el.focus()
    if (el instanceof HTMLInputElement) el.select()
  },
})

const store = useAppStore()
store.loadAll()

app.mount('#app')