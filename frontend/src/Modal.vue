<template>
    <ShoppingCart v-if="name === 'ShoppingCart'" />
    <Purchase v-else-if="name === 'Purchase'" />
</template>

<script setup>
import ShoppingCart from '@/views/ShoppingCart.vue';
import Purchase from '@/views/modals/purchase/Purchase.vue';

const props = defineProps({
  name: String
})

</script>
<style lang="less"></style>