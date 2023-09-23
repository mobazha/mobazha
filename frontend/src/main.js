import { createApp } from 'vue';
import { createStore } from 'vuex';
import ElementPlus from 'element-plus';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import 'element-plus/dist/index.css';
import './assets/scss/main.scss';
import { TUIComponents, TUICore } from './TUIKit';

import app from '../backbone/app';

import App from './App.vue';
import baseVw from './mixins/baseVw';
import Router from './router/index';
import components from './components/global';

import * as templateHelpers from '../backbone/utils/templateHelpers';

import cart from './store/cart.module';

// init TUIKit
const TUIKit = TUICore.init({});
// TUIKit add TUIComponents
TUIKit.use(TUIComponents);

window.TUIKit = TUIKit;

window.app = app;

function mountVueApp(container) {
  const vueApp = createApp(App);
  vueApp.config.productionTip = false;

  vueApp.use(ElementPlus);
  for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    vueApp.component(key, component);
  }

  // components
  for (const i in components) {
    vueApp.component(i, components[i]);
  }

  vueApp.config.globalProperties.templateHelpers = { ...templateHelpers };
  vueApp.mixin(baseVw);

  const store = createStore({
    modules: {
      cart,
    },
  });

  return vueApp.use(Router).use(store).mount(container);
}
window.vueApp = mountVueApp('#appFrame');
