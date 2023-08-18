import { createApp } from 'vue';
import { createStore } from 'vuex';

import ElementPlus from 'element-plus';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import 'element-plus/dist/index.css';
import ShoppingCart from './components/ShoppingCart.vue';

import Chat from './components/Chat.vue'
import { TUIComponents, TUICore, genTestUserSig } from './TUIKit';
// import TUICallKit
import { TUICallKit } from '@tencentcloud/call-uikit-vue';
import MyChatSDK from './TUIKit/myChatSDK';

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

export function mountChat(container) {
  const SDKAppID = 1771000181; // Your SDKAppID
  const secretKey =
    '340f95d79d6810703504d6b9008c901a20070905605f38ff5a49dd23811f85b6'; //Your secretKey
  const userID = 'test123'; // User ID

  const chat = createApp(Chat);
  chat.config.productionTip = false;

  // components
  for (const i in components) {
    chat.component(i, components[i]);
  }

  const store = createStore({
    modules: {
      products,
    },
  });

  // init TUIKit
  const TUIKit = TUICore.init({
    SDKAppID,
  });
  // TUIKit add TUIComponents
  TUIKit.use(TUIComponents);
  // TUIKit add TUICallKit
  TUIKit.use(TUICallKit);

  // // login TUIKit
  // TUIKit.login({
  //   userID: userID,
  //   userSig: genTestUserSig({
  //     SDKAppID,
  //     secretKey,
  //     userID,
  //   }).userSig, // The password with which the user logs in to IM. It is the ciphertext generated by encrypting information such as userID.For the detailed generation method, see Generating UserSig
  // });

  MyChatSDK.emit(TUIKit.TIM.EVENT.SDK_READY, {});

  chat.use(Router).use(TUIKit).use(store).mount(container);
}
