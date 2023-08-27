<template>
  <div class="cryptoTradingPairWrap">

    {{ ob.crypto.tradingPair(tradingPairOptions) }}

    <div :class="exchangeRateClass">{{ exchangeRateLine }}</div>

  </div>
</template>

<script>
export default {
  data () {
    return {
      tradingPairClass: 'cryptoTradingPairLg',
      fromCur: '',
      fromCurAmount: 1,
      toCur: '',
      localCurrency: app.settings.get('localCurrency'),
      // If passing this in, it should be a string or a function. If it's a function
      // it will be passed the state and coin(s) with missing exchange rates and it
      // should return a string.
      noExchangeRateTip: coinsMissingRates => (
        app.polyglot.t('cryptoTradingPair.tipMissingExchangeRate', {
          coins: coinsMissingRates.join(', '),
        })
      ),
      exchangeRateUnavailable: false,
      iconClass: 'ion-alert-circled clrTAlert',
      truncateCurAfter: 8,
    };
  },
  props: {
    feeLevel: {
      type: String,
    },
    exchangeRateClass: {
      type: String,
    },
  },
  methods: {

    loadData () {
      const tradingPairOptions = Object.assign({
        className: ob.tradingPairClass,
        fromCur: ob.fromCur,
        toCur: ob.toCur,
      }, ob);

      if (!ob.fromRateUnavailable && !ob.toRateUnavailable) {
        const formattedFromCurAmount = new Intl.NumberFormat(ob.localCurrency, {
          minimumFractionDigits: 0,
          maximumFractionDigits: 8,
        }).format(ob.fromCurConvertedAmount);

        this.exchangeRateLine = ob.polyT('cryptoConversionPairing', {
          fromPairing: ob.currencyMod.formatCurrency(ob.fromCurAmount, ob.fromCur, { useCryptoSymbol: false }),
          toPairing: ob.currencyMod.formatCurrency(ob.toCurAmount, ob.toCur, { useCryptoSymbol: false }),
          fromCurAmount: formattedFromCurAmount,
        });
      } else {
        let icon = `<span class="${ob.iconClass}"></span>`;

        if (ob.noExchangeRateTip) {
          icon = `<span class="toolTip" data-tip="${ob.noExchangeRateTip}">${icon}</span>`;
        }

        this.exchangeRateLine = ob.polyT('cryptoTradingPair.cryptoConversionPairingNoToCurExchangeRate', {
          fromPairing: ob.currencyMod.formatCurrency(ob.fromCurAmount, ob.fromCur, { useCryptoSymbol: false }),
          icon,
        });
      }

    }
  }
};


</script>
<style lang="scss" scoped>
</style>