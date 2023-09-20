<template>
    <ShoppingCart :ref="name" v-if="name === 'ShoppingCart'" :options="options" />
    <Purchase :ref="name" v-else-if="name === 'Purchase'" :options="options" />
    <OrderDetail :ref="name" v-else-if="name === 'OrderDetail'" :options="options" />
    <Wallet :ref="name" v-else-if="name === 'Wallet'" :options="options" />
</template>

<script>
import ShoppingCart from '@/views/ShoppingCart.vue';
import Purchase from '@/views/modals/purchase/Purchase.vue';
import OrderDetail from '@/views/modals/orderDetail/OrderDetail.vue';
import Wallet from '@/views/modals/wallet/Wallet.vue';

export default {
  components: {
    ShoppingCart,
    Purchase,
    OrderDetail,
    Wallet,
  },
  props: {
    name: {
      type: String,
      default: '',
    },
    options: {
      type: Object,
      default: {},
    },
  },
  methods: {
    getActiveRef() {
      return this.$refs[this.name];
    },
  },
}
</script>
