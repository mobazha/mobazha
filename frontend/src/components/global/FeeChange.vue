<template>
  <div>
    {{ ob.polyT('feeChangeWidget.label') }}
    <span class="txB">{{ ob.polyT(`feeLevels.${feeLevel}`) }}</span>
    <button :class="`js-changeFee btnAsLink clrT2`">{{ ob.polyT('feeChangeWidget.btnChange') }}</button>
  </div>
</template>

<script setup>
const props = defineProps({
  feeLevel: String,
})
</script>
<style lang="scss" scoped>
</style>