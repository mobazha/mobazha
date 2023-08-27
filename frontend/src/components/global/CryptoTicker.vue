<template>
  <div>

  <!-- We won't show anything if there is no displayRate is set. It's likely the required exchange
    rates weren't available to make the calculation. -->
  <div v-if="ob.displayRate" class="flexVCent clrP clrBr clrSh3 contentBox padSm toolTipNoWrap" :data-tip="tip">
    <%= ob.crypto.cryptoIcon({
      className: 'margRSm',
      code: ob.coinType,
    }) %>
    <strong class="tx5">{{ ob.displayRate }}</strong>
  </div>

</div>
</template>

<script setup>
const props = defineProps({
  phase: String,
})

const tip = ob.polyT('cryptoTicker.currentPrice', {
  cur: ob.polyT(`cryptoCurrencies.${ob.coinType}`, { _: ob.coinType }),
});

</script>
<style lang="scss" scoped>
</style>