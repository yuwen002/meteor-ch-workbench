import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useAppStore } from './stores/app'
import './style.css'

const app = createApp(App)
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