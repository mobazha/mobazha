<template>
  <div>

    <div class="flexVCent gutterHLg row">
      <div class="js-cryptoTitle"></div>
      <div class="flexExpand">
        <div class="flexVCent gutterHLg">
          <label for="cryptoAmount" class="clrT txB required">{{ ob.polyT('purchase.cryptoAmount') }}</label>
          <div class="inputSelect">
            <input type="text" class="clrBr clrP clrSh2" name="quantity" id="cryptoAmount" :value="ob.quantity" placeholder="0.0000" size="8">
            <template v-if="ob.displayCurrency !== ob.listing.metadata.coinType">
              <select id="cryptoAmountCurrency" class="clrBr clrP nestInputRight">
                <option
                  v-for="(cur, j) in [ob.listing.metadata.coinType, ob.displayCurrency]"
                  :key="j"
                  :value="cur"
                  :selected="cur === ob.cryptoAmountCurrency">{{ cur }}</option>
              </select>
            </template>
          </div>
        </div>
      </div>
      <div class="pad flexNoShrink">
        <b>
          {{ ob.currencyMod.convertAndFormatCurrency(totalPrice, pricingCurrency, ob.displayCurrency) }}
        </b>
      </div>
    </div>
    <hr class="clrBr rowLg" />
    <div class="rowSm">
      <label class="h4 flexExpand required" for="purchaseCryptoAddress">{{ heading }}</label>
    </div>
    <div class="js-items-paymentAddress-errors"></div>
    <input type="text"
      id="purchaseCryptoAddress"
      :value="ob.items[0].paymentAddress"
      :placeholder="placeholder"
      class="clrBr clrP rowSm"
      :maxlength="ob.itemConstraints.maxPaymentAddressLength" />
    <div class="txSm clrT2">{{ helper }}</div>
  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {},
	  },
  },
  data () {
    return {
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    // when multiple listings are supported, the prices array will have one price object for each
    totalPrice () {
      return this.prices[0].price + ob.prices[0].vPrice;
    },
    pricingCurrency () {
      return ob.listingPrice.currencyCode;
    },
    coinName () {
      const coinType = ob.listing.metadata.coinType;
      const coinTranslationKey = `cryptoCurrencies.${coinType}`;
      return ob.polyT(coinTranslationKey) === coinTranslationKey ? coinType : ob.polyT(coinTranslationKey);
    },
    heading () {
      return ob.polyT('purchase.cryptoAddressHeading', { coinType: this.coinName, });
    },
    placeholder () {
      return ob.polyT('purchase.cryptoAddressPlaceholder', { coinType: this.coinName, });
    },
    helper () {
      const warning = `<b>${ob.polyT('purchase.cryptoAddressHelperWarning')}</b>`;
      return ob.polyT('purchase.cryptoAddressHelper', {
        name: ob.vendor.name,
        coinType: this.coinName,
        warning,
      });
    },
  },
  methods: {

  }
}
</script>
<style lang="scss" scoped></style>
