<template>
  <div class="cryptoTicker">
    <!-- We won't show anything if there is no displayRate is set. It's likely the required exchange
    rates weren't available to make the calculation. -->
    <template v-if="displayRate">
      <div class="flexVCent clrP clrBr clrSh3 contentBox padSm toolTipNoWrap" :data-tip="tip">
        <CryptoIcon :code="coinType" className="margRSm"/>
        <strong class="tx5">{{ displayRate }}</strong>
      </div>
    </template>

  </div>
</template>

<script>
import app from '../../../backbone/app';
import {
  convertAndFormatCurrency,
  events as currencyEvents,
} from '../../../backbone/utils/currency';


export default {
  props: {
    coinType: {
      type: String,
      default: '',
    },
  },
  data () {
    return {
      tip: '',

      rateToggle: false,
    };
  },
  created () {
    this.initEventChain();

    this.listenTo(currencyEvents, 'exchange-rate-change', e => {
      if (e.changed.includes(this.displayCur) ||
        e.changed.includes(this.coinType)) {
        this.rateToggle = !this.rateToggle;
      }
    });
  },
  mounted () {
  },
  computed: {
    displayCur () {
      return app.settings.get('localCurrency') || 'USD';
    },
    displayRate () {
      let access = this.rateToggle;

      if (this.coinType === this.displayCur) {
        return null;
      }

      let rate = null;

      try {
        rate = convertAndFormatCurrency(1, this.coinType, this.displayCur, { skipConvertOnError: false });
      } catch (e) {
        // pass
      }

      return rate;
    },
  },
  methods: {
  }
}
</script>
<style lang="scss" scoped></style>
