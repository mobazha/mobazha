<template>
  <!--  ${ob.active ? 'active' : ''} ${!ob.clientSupported ? 'clientUnsupported' : ''} -->
  <li :class="`coinNavItem flexVCent gutterHSm lineHeight1 tx4 clrT2`">
    <CryptoIcon :code="mnCode" className="flexNoShrink" />
    <div :class="`flexExpand lineHeight1 ${ob.active ? 'clrT' : ''} coinName`">{{ ob.polyT(`cryptoCurrencies.${mnCode}`, { _: mnCode }) }}</div>
    <div :class="`${ob.balance > 0 ? 'clrTEm' : ''} flexNoShrink balanceText`">
      <template v-if="ob.clientSupported">
        <div class="flexVCent flexHRight">
          <i v-if="ob.active" class="ion-arrow-right-c clrT2 activeBalanceIcon"></i>
          <span v-else-if="ob.balance > 0" class="clrTEm txB">{{ formattedBalance }}</span>
          <template v-else>{{ formattedBalance }}</template>
        </div>
      </template>
      <template v-else>
        <span class="toolTip" :data-tip="ob.polyT('wallet.coinNav.unsupportedCurTip')">
          <i class="ion-help-circled"></i>
        </span>
      </template>
    </div>
  </li>
</template>

<script>
import app from '../../../../backbone/app';
import { NoExchangeRateDataError } from '../../../../backbone/utils/currency';

export default {
  props: {
    options: {
      type: Object,
      default: {
      },
    },
  },
  data() {
    return {
      _state: {
        active: false,
        code: '',
        name: '',
        balance: undefined,
        clientSupported: false,

        displayCur: (app && app.settings && app.settings.get('localCurrency')) || 'USD',
      }
    };
  },
  created() {
    this.loadData(this.options);
  },
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
        NoExchangeRateDataError,
      };
    },
    displayCur() {
      return (app && app.settings && app.settings.get('localCurrency')) || 'USD';
    },
    formattedBalance() {
      let convertedCurrency;
      try {
        convertedCurrency = this.ob.currencyMod.convertCurrency(this.ob.balance, this.mnCode, this.ob.displayCur);
      } catch (e) {
        if (e instanceof NoExchangeRateDataError) {
          // pass - we'll just show the unconverted amount if the exchange rate data isn't
          // available
        }
      }

      let formattedBalance = '';

      if (typeof this.ob.balance === 'number') {
        formattedBalance =
          convertedCurrency === undefined
            ? this.ob.currencyMod.formatCurrency(this.ob.balance, this.mnCode, { maxDisplayDecimals: 4 })
            : this.ob.currencyMod.formatCurrency(convertedCurrency, this.ob.displayCur, {
                maxDisplayDecimals: ob.currencyMod.isFiatCur(this.ob.displayCur) ? 2 : 4,
              });
      }
      return formattedBalance;
    },
    mnCode() {
      return this.ob.crypto.ensureMainnetCode(this.ob.code);
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        initialState: {
          active: false,
          displayCur: (app && app.settings && app.settings.get('localCurrency')) || 'USD',
          ...options.initialState,
        },
      };

      if (!opts.initialState || typeof opts.initialState.code !== 'string' || !opts.initialState.code) {
        throw new Error('Please provide a code as a non-empty string in the initial state');
      }

      if (!opts.initialState || typeof opts.initialState.name !== 'string' || !opts.initialState.name) {
        throw new Error('Please provide a name as a non-empty string in the initial state');
      }

      this.baseInit(opts);
    },
  },
};
</script>
<style lang="scss" scoped></style>
