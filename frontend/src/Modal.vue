<template>
    <ShoppingCart v-if="name === 'ShoppingCart'" />
    <Purchase v-else-if="name === 'Purchase'" />
    <OrderDetail v-else-if="name === 'OrderDetail'" />
    <Wallet v-else-if="name === 'Wallet'" />
</template>

<script setup>
import ShoppingCart from '@/views/ShoppingCart.vue';
import Purchase from '@/views/modals/purchase/Purchase.vue';
import OrderDetail from '@/views/modals/orderDetail/OrderDetail.vue';
import Wallet from '@/views/modals/wallet/Wallet.vue';

const props = defineProps({
  name: String
})

</script>
<style lang="less"></style>