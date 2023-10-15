import { createApp } from 'vue';
import 'element-plus/dist/index.css';

// import './assets/scss/main.scss';

import Chat from './components/Chat.vue';

import './assets/global.less';
import components from './components/global';
import Router from './router/index';

export function mountChat(container, conversationID) {
  const chat = createApp(Chat, { conversationID });
  chat.config.productionTip = false;

  // components
  for (const i in components) {
    chat.component(i, components[i]);
  }

  return chat.use(Router).use(window.TUIKit).mount(container);
}
