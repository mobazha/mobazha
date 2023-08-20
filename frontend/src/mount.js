import { createApp } from 'vue';
import { createStore } from 'vuex';

import ElementPlus from 'element-plus';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import 'element-plus/dist/index.css';
import ShoppingCart from './components/ShoppingCart.vue';

import Chat from './components/Chat.vue'


import './assets/global.less';
import components from './components/global';
import products from './store/products.module';
import Router from './router/index';

export function mountShoppingCart() {
  const shoppingCart = createApp(ShoppingCart);
  shoppingCart.config.productionTip = false;

  shoppingCart.use(ElementPlus);
  for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    shoppingCart.component(key, component);
  }
  // components
  for (const i in components) {
    shoppingCart.component(i, components[i]);
  }
  const store = createStore({
    modules: {
      products,
    },
  });

  shoppingCart.use(Router).use(store).mount('#shoppingCart');
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
