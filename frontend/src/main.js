import { createApp } from 'vue';
import { createStore } from 'vuex'
import App from './App.vue';
import './assets/global.less';
import components from './components/global';
import products from './store/products.module'
import Router from './router/index';

import sifter from 'sifter';
import microplugin from 'microplugin';
import $ from "jquery";

window.jQuery = window.$ = $;
window.Sifter = sifter;
window.MicroPlugin = microplugin;

const app = createApp(App)
app.config.productionTip = false

// components
for (const i in components) {
  app.component(i, components[i])
}

const store = createStore({
  modules: {
    products
  }
})

$.vueRouter = Router;
$.vueStore = store;
app.use(Router).use(store).mount('#app')
