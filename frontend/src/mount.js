import { createApp } from 'vue';
import { createStore } from 'vuex';

import ElementPlus from 'element-plus';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import 'element-plus/dist/index.css';

import Modal from './Modal.vue'
import baseVw from './mixins/baseVw';

// import './assets/scss/main.scss';

import Chat from './components/Chat.vue';

import OrderDetail from './views/modals/orderDetail/OrderDetail.vue'

import './assets/global.less';
import components from './components/global';
import cart from './store/cart.module';
import Router from './router/index';

import * as templateHelpers from '../backbone/utils/templateHelpers';

export function mountVueModal(container, name, options) {
  const vueModal = createApp(Modal, { name, options });
  vueModal.config.productionTip = false;

  vueModal.mixin(baseVw);
  vueModal.config.globalProperties.templateHelpers = {...templateHelpers};

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

  return typeof vueModal.getActiveRef === 'function' ? vueModal.getActiveRef() : vueModal;
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
