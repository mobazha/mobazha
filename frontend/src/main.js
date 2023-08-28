import { createStore } from 'vuex'

import { TUIComponents, TUICore } from './TUIKit';

import app from '../backbone/app';

import './assets/global.less';
import products from './store/products.module'
import Router from './router/index';

import sifter from 'sifter';
import microplugin from 'microplugin';
import $ from "jquery";

import * as templateHelpers from '../backbone/utils/templateHelpers';

window.jQuery = window.$ = $;
window.Sifter = sifter;
window.MicroPlugin = microplugin;
window.app = app;

window.templateHelpers = templateHelpers;

const store = createStore({
  modules: {
    products
  }
})

$.vueRouter = Router;
$.vueStore = store;

// init TUIKit
const TUIKit = TUICore.init({});
// TUIKit add TUIComponents
TUIKit.use(TUIComponents);

window.TUIKit = TUIKit;
