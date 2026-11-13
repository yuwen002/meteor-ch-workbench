import { createRouter, createWebHashHistory } from 'vue-router'
import QueryEditor from '../views/QueryEditor.vue'

const routes = [
  { path: '/', redirect: '/query' },
  { path: '/query', name: 'query', component: QueryEditor },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router