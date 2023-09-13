import { createApp } from 'vue';
import { createStore } from 'vuex';

import ElementPlus from 'element-plus';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import 'element-plus/dist/index.css';

import App from './App.vue'
import Modal from './Modal.vue'

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

  // components
  for (const i in components) {
    vueApp.component(i, components[i]);
  }
  const store = createStore({
    modules: {
      cart,
    },
  });

  vueApp.use(Router).use(store).mount(container);

  return vueApp;
}

export function mountVueModal(container, name) {
  const vueModal = createApp(Modal, { name });
  vueModal.config.productionTip = false;

  vueModal.config.globalProperties.ob = {...templateHelpers};

  vueModal.use(ElementPlus);
  for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    vueModal.component(key, component);
  }
  // components
  for (const i in components) {
    vueModal.component(i, components[i]);
  }
  const store = createStore({
    modules: {
      cart,
    },
  });

  vueModal.use(Router).use(store).mount(container);

  return vueModal;
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
