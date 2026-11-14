import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/query' },
  { path: '/query', name: 'query', component: () => import('../views/QueryEditor.vue') },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router