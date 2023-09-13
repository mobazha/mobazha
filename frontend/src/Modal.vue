<template>
    <ShoppingCart v-if="name === 'ShoppingCart'" />
    <Purchase v-else-if="name === 'Purchase'" />
    <orderDetail v-else-if="name === 'orderDetail'" />
</template>

<script setup>
import ShoppingCart from '@/views/ShoppingCart.vue';
import Purchase from '@/views/modals/purchase/Purchase.vue';
import orderDetail from '@/views/modals/orderDetail/orderDetail.vue';

const props = defineProps({
  name: String
})

</script>
<style lang="less"></style>