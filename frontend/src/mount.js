import { createApp } from 'vue';
import { createStore } from 'vuex';

import ElementPlus from 'element-plus';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import 'element-plus/dist/index.css';

import App from './App.vue'

// import './assets/scss/main.scss';

import Chat from './components/Chat.vue';

import OrderDetail from './views/modals/orderDetail/OrderDetail.vue'

import './assets/global.less';
import components from './components/global';
import cart from './store/cart.module';
import Router from './router/index';

import * as templateHelpers from '../backbone/utils/templateHelpers';

export function mountVueApp(container) {
  const vueApp = createApp(App);
  vueApp.config.productionTip = false;

  vueApp.config.globalProperties.ob = {...templateHelpers};

  vueApp.use(ElementPlus);
  for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    vueApp.component(key, component);
  }
  // components
  for (const i in components) {
    vueApp.component(i, components[i]);
  }
  const store = createStore({
    modules: {
      cart,
    },
  });

  return vueApp.use(Router).use(store).mount(container);
}

export function mountChat(container, conversationID) {
  const chat = createApp(Chat, { conversationID });
  chat.config.productionTip = false;

  // components
  for (const i in components) {
    chat.component(i, components[i]);
  }

  return chat.use(Router).use(window.TUIKit).mount(container);
}

export function mountOrderDetail(container, options) {
  const orderDetail = createApp(OrderDetail, options);
  orderDetail.config.productionTip = false;

  // components
  for (const i in components) {
    orderDetail.component(i, components[i]);
  }

  return orderDetail.mount(container);
}
