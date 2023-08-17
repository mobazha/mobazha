import { createApp } from 'vue';
import { createStore } from 'vuex'

import ShoppingCart from './components/ShoppingCart.vue';

import './assets/global.less';
import components from './components/global';
import products from './store/products.module'
import Router from './router/index';

export function moutShoppingCart() {

  const shoppingCart = createApp(ShoppingCart)
  shoppingCart.config.productionTip = false

  // components
  for (const i in components) {
    shoppingCart.component(i, components[i])
  }

  const store = createStore({
    modules: {
      products
    }
  })

  shoppingCart.use(Router).use(store).mount('#shoppingCart')
}


