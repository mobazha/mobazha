import { createRouter, createWebHistory } from 'vue-router'
import routerMap from './routerMap'

const Router = createRouter({
  history: createWebHistory(),
  routes: routerMap,
})


export default Router
