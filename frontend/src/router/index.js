import { createRouter, createWebHistory } from 'vue-router'
import routerMap from './routerMap'
import * as casdoor from '../utils/casdoor';

const Router = createRouter({
  history: createWebHistory(),
  routes: routerMap,
})

Router.beforeEach((to, from, next) => {
  if (!import.meta.env.VITE_APP && to.name !== 'Callback' && !casdoor.isLoggedIn()) {
    window.location.href = casdoor.getSigninUrl();
  } else {
    next();
  }
});

export default Router
