<template>
  <!--  ${ob.active ? 'active' : ''} ${!ob.clientSupported ? 'clientUnsupported' : ''} -->
  <li :class="`coinNavItem flexVCent gutterHSm lineHeight1 tx4 clrT2`">
    <CryptoIcon :code="mnCode" className="flexNoShrink" />
    <div :class="`flexExpand lineHeight1 ${ob.active ? 'clrT' : ''} coinName`">
      <div>
        {{ ob.polyT(`cryptoCurrencies.${mnCode}`, { _: mnCode }) }}
      </div>
      <div v-if="mainChainName" class="coin-label">{{mainChainName}}</div>
    </div>

    <div :class="`${ob.balance > 0 ? 'clrTEm' : ''} flexNoShrink balanceText`">
      <template v-if="ob.clientSupported">
        <div class="flexVCent flexHRight">
          <span v-if="ob.balance > 0" class="clrTEm txB">{{ formattedBalance }}</span>
          <i v-if="ob.active" class="ion-arrow-right-c clrT2 activeBalanceIcon"></i>
        </div>
      </template>
      <template v-else>
        <span class="toolTip" :data-tip="ob.polyT('wallet.coinNav.unsupportedCurTip')">
          <i class="ion-help-circled"></i>
        </span>
      </template>
      <span
        v-if="externalEnabled"
        class="extflag txB flexHRight toolTip"
        :data-tip="ob.polyT('wallet.coinNav.extEnabled', { cur: ob.polyT(`cryptoCurrencies.${mnCode}`) })"
        >Ext-enabled</span
      >
    </div>
  </li>
</template>

<script>
import bigNumber from 'bignumber.js';
import app from '../../../../backbone/app';
import { NoExchangeRateDataError } from '../../../../backbone/utils/currency';
import { getCurrencyByCode } from '../../../../backbone/data/walletCurrencies';

export default {
  props: {
    options: {
      type: Object,
      default: {
        active: false,
        code: '',
        name: '',
        balance: undefined,
        clientSupported: false,
      },
    },
  },
  data() {
    return {};
  },
  created() {},
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this.options,
        NoExchangeRateDataError,
      };
    },
    displayCur() {
      return (app && app.settings && app.settings.get('localCurrency')) || 'USD';
    },
    formattedBalance() {
      const ob = this.ob;

      let convertedCurrency;
      try {
        convertedCurrency = ob.currencyMod.convertCurrency(ob.balance, this.mnCode, this.displayCur);
      } catch (e) {
        if (e instanceof NoExchangeRateDataError) {
          // pass - we'll just show the unconverted amount if the exchange rate data isn't
          // available
        }
      }

      let formattedBalance = '';

      if (typeof ob.balance === 'number' || ob.balance instanceof bigNumber) {
        formattedBalance =
          convertedCurrency === undefined
            ? ob.currencyMod.formatCurrency(ob.balance, this.mnCode, { maxDisplayDecimals: 4 })
            : ob.currencyMod.formatCurrency(convertedCurrency, this.displayCur, {
                maxDisplayDecimals: ob.currencyMod.isFiatCur(this.displayCur) ? 2 : 4,
              });
      }
      return formattedBalance;
    },
    mnCode() {
      const ob = this.ob;

      return ob.code && ob.crypto.ensureMainnetCode(ob.code);
    },
    mainChainName() {
      const coinData = getCurrencyByCode(this.ob.code);
      if (!coinData || !coinData.mainChain) {
        return '';
      }

      const mainChainData = getCurrencyByCode(coinData.mainChain);

      return mainChainData.chainName;
    },
    externalEnabled() {
      let externalPaymentAddresses = app.settings.get('externalPaymentAddresses') || {};
      const { code } = this.options;
      const lastAddress = externalPaymentAddresses[code]?.address;
      const enabled = externalPaymentAddresses[code]?.enable;

      return lastAddress && enabled;
    },
  },
  methods: {},
};
</script>
<style lang="scss" scoped>
.extflag {
  color: #cc920b;
  font-size: 14px;
  margin-top: 3px;
}
.coinNavItem {
  padding: 10px 15px;
}
.coinName {
  flex: 1;
}
.cryptoIcon {
  width: 32px;
  height: 32px;
}
.coin-label {
  display: inline-block;
  font-size: 12px;
  background: #f2f2f9;
  padding: 3px;
  border-radius: 5px;
  white-space: nowrap;
  margin-top: 5px;
  color: #333;
  // max-width: 80px;
  // overflow: hidden;
  // text-overflow: ellipsis;
}
</style>
