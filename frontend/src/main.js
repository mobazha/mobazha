import { createApp } from 'vue';
import App from './App.vue';
import './assets/global.less';
import components from './components/global';
import Router from './router/index';

import sifter from 'sifter';
import microplugin from 'microplugin';
import $ from "jquery";

window.jQuery = $;
window.Sifter = sifter;
window.MicroPlugin = microplugin;

const app = createApp(App)
app.config.productionTip = false

// components
for (const i in components) {
  app.component(i, components[i])
}

// app.use(Router).mount('#app')
