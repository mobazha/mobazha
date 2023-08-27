<template>
  <div>

<{{ ob.wrappingTag }} :class="`${ob.wrappingClass} ${ob.priceModifier !== 0 ? (ob.priceModifier > 0 ? 'clrTPriceAboveMarket' : 'clrTPriceBelowMarket') : ''}`">
  <span>
    {{
      ob.currencyMod.convertAndFormatCurrency(
        ob.priceAmount,
        ob.priceCurrencyCode,
        ob.displayCurrency,
        ob.convertAndFormatOpts
      )
    }}&nbsp;<span class="priceSymbol">
      <div v-if="ob.priceModifier !== 0">
        <span v-if="ob.priceModifier > 0" class="icon ion-arrow-up-c"></span>
        <span v-else class="icon ion-arrow-down-c"></span>
      </div>
      <span v-else class="ion-checkmark clrTEm"></span>
    </span>
  </span>
  <span :class="`marketRelativity ${ob.marketRelativityClass} ${ob.priceModifier === 0 ? 'clrT2' : ''}`">
    {{ marketPriceLine }}
  </span>
</{{ ob.wrappingTag }}>


</div>
</template>

<script setup>
const props = defineProps({
  phase: String,
})

let marketPriceLine = ob.polyT('cryptoPriceAtMarket');
if (ob.priceModifier > 0) {
  marketPriceLine = ob.polyT('cryptoPriceAboveMarket', { amount: ob.priceModifier });
} else if (ob.priceModifier < 0) {
  marketPriceLine = ob.polyT('cryptoPriceBelowMarket', { amount: ob.priceModifier });
}

</script>
<style lang="scss" scoped>
</style>