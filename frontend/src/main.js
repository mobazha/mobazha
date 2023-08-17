import { createStore } from 'vuex'

import app from '../backbone/app';

import './assets/global.less';
import products from './store/products.module'
import Router from './router/index';

import sifter from 'sifter';
import microplugin from 'microplugin';
import $ from "jquery";

window.jQuery = window.$ = $;
window.Sifter = sifter;
window.MicroPlugin = microplugin;
$.app = app;

const store = createStore({
  modules: {
    products
  }
})

$.vueRouter = Router;
$.vueStore = store;
