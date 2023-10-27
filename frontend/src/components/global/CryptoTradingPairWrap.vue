<template>
  <div class="cryptoTradingPairWrap">
    <CryptoTradingPair :options="tradingPairOptions" />

    <div :class="ob.exchangeRateClass">
      <template v-if="!ob.fromRateUnavailable && !ob.toRateUnavailable">
        {{ `${fromPairing} = ${toPairing} (${ob.fromCurAmount})` }}
      </template>
      <template v-else>
        {{ fromPairing }} =
        <span v-if="noExchangeRateTip" class="toolTip" :data-tip="noExchangeRateTip">
          <span :class="ob.iconClass"></span>
        </span>
        <span v-else :class="ob.iconClass"></span>
      </template>
    </div>
  </div>
</template>

<script>
/**
 * Will render a a combination of two currenciees indicating that one is being
 * traded for the other (e.g. <btc-icon> BTC > <zec-icon> ZEC), followed by an
 * optional line of text indicating the exchange rate between the two currencies
 * (the view will update if the exchange rate changes). This differs from
 * renderCryptoTradingPair in the crypto util module (which is also the
 * ob.crypto.tradingPair template helper) in that the latter is just a simple display
 * of two currencies being traded for one another. If you do not need to display an
 * exchange rate and your currencies won't change dynamically, the latter might be
 * slightly less boilerplate to implement.
 */

import app from '../../../backbone/app';
import {
  getExchangeRate,
  events as currencyEvents,
} from '../../../backbone/utils/currency';
import { ensureMainnetCode } from '../../../backbone/data/walletCurrencies';
import CryptoTradingPair from './CryptoTradingPair.vue'


export default {
  components: {
    CryptoTradingPair,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      defaultOptions: {
        tradingPairClass: 'cryptoTradingPairLg',
        exchangeRateClass: '',
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
      },
      newOptions: {},
      triggerUpdate: 0,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      let access = this.triggerUpdate;

      let newOptions = {
        ...this.defaultOptions,
        ...this.options,
      }

      if (typeof newOptions.fromCur === 'string') {
        newOptions.fromCur = ensureMainnetCode(newOptions.fromCur);

        if (newOptions.fromCur > newOptions.truncateCurAfter) {
          newOptions.fromCur = `${newOptions.fromCur.slice(0, newOptions.truncateCurAfter)}…`;
        }
      }

      if (typeof newOptions.toCur === 'string') {
        newOptions.toCur = ensureMainnetCode(newOptions.toCur);

        if (newOptions.toCur > newOptions.truncateCurAfter) {
          newOptions.toCur = `${newOptions.toCur.slice(0, newOptions.truncateCurAfter)}…`;
        }
      }

      newOptions = {
        ...newOptions,
        ...this.getConversionState(newOptions.fromCur, newOptions.toCur, newOptions.fromCurAmount),
      };

      this.newOptions = newOptions;

      return {
        ...this.templateHelpers,
        ...this.newOptions,
      };
    },
    tradingPairOptions () {
      const ob = this.ob;
      return {
        className: ob.tradingPairClass,
        fromCur: ob.fromCur,
        toCur: ob.toCur,
      };
    },
    fromPairing () {
      const ob = this.ob;
      return ob.currencyMod.formatCurrency(ob.fromCurAmount, ob.fromCur, { useCryptoSymbol: false });
    },
    toPairing () {
      const ob = this.ob;
      return ob.currencyMod.formatCurrency(ob.toCurAmount, ob.toCur, { useCryptoSymbol: false });
    },
    formattedFromCurAmount () {
      const ob = this.ob;
      return new Intl.NumberFormat(ob.localCurrency, {
        minimumFractionDigits: 0,
        maximumFractionDigits: 8,
      }).format(ob.fromCurConvertedAmount);
    },
    noExchangeRateTip () {
      const state = this.newOptions;
      const coinsMissingRates = [];

      if (state.toRateUnavailable) coinsMissingRates.push(state.toCur);
      if (state.fromRateUnavailable) coinsMissingRates.push(state.fromCur);

      let noExchangeRateTip;

      if (coinsMissingRates.length) {
        if (typeof state.noExchangeRateTip === 'function') {
          noExchangeRateTip = state.noExchangeRateTip(coinsMissingRates, state);
        } else if (typeof state.noExchangeRateTip === 'string') {
          noExchangeRateTip = state.noExchangeRateTip;
        }
      }
      return noExchangeRateTip;
    },
  },
  methods: {
    loadData () {

      this.listenTo(currencyEvents, 'exchange-rate-change', e => {
        if (e.changed.includes(this.options.toCur) || e.changed.includes(this.options.fromCur)) {
          this.triggerUpdate += 1;
        }
      });
    },

    getConversionState (fromCur, toCur, fromCurAmount) {
      const fromCurRate = getExchangeRate(fromCur);
      const toCurRate = getExchangeRate(toCur);

      return {
        toCurAmount: (toCurRate / fromCurRate) * fromCurAmount,
        fromCurConvertedAmount: (fromCurRate / toCurRate) * fromCurAmount,
        fromRateUnavailable: fromCurRate === undefined,
        toRateUnavailable: toCurRate === undefined,
      };
    },

  }
}
</script>
<style lang="scss" scoped></style>
