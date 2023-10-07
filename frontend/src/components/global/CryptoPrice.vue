<template>
  <div
    :is="ob.wrappingTag"
    :class="`${ob.wrappingClass} ${ob.priceModifier !== 0 ? (ob.priceModifier > 0 ? 'clrTPriceAboveMarket' : 'clrTPriceBelowMarket') : ''}`"
  >
    <span
      >{{ ob.currencyMod.convertAndFormatCurrency(ob.priceAmount, ob.priceCurrencyCode, ob.displayCurrency, ob.convertAndFormatOpts) }}&nbsp;
      <span class="priceSymbol" v-html="priceSymbol"></span>
    </span>
    <span :class="`marketRelativity ${ob.marketRelativityClass} ${ob.priceModifier === 0 ? 'clrT2' : ''}`">
      {{ marketPriceLine }}
    </span>
  </div>
</template>
<script>
export default {
  props: {
    options: {
      type: Object,
      default: {
        priceAmount: 0,
        priceCurrencyCode: '',
        displayCurrency: '',
        priceModifier: 0,
        wrappingTag: '',
        wrappingClass: '',
        convertAndFormatOpts: {
          maxDisplayDecimals: 0,
        },
      },
    },
  },
  data() {
    return {
    };
  },
  created() {
    this.loadData(this.options);
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.options,
      };
    },
    colorClass() {
      const ob = this.ob;
      if (ob.priceModifier !== 0) return '';
      return ob.priceModifier > 0 ? 'clrTPriceAboveMarket' : 'clrTPriceBelowMarket';
    },
    priceSymbol() {
      const ob = this.ob;
      if (ob.priceModifier !== 0) return '(<span class="ion-checkmark clrTEm"></span>)';
      return ob.priceModifier > 0 ? `(<span class="icon ion-arrow-up-c"></span>)` : `(<span class="icon ion-arrow-down-c"></span>)`;
    },
    marketPriceLine() {
      const ob = this.ob;
      if (ob.priceModifier > 0) {
        return ob.polyT('cryptoPriceAboveMarket', { amount: ob.priceModifier });
      }
      if (ob.priceModifier < 0) {
        return ob.polyT('cryptoPriceBelowMarket', { amount: ob.priceModifier });
      }
      return ob.polyT('cryptoPriceAtMarket');
    },
  },
};
</script>
<style lang="scss" scoped></style>
