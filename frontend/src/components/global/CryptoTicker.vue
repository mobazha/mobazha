<template>
  <div class="cryptoTicker">
    <!-- We won't show anything if there is no displayRate is set. It's likely the required exchange
    rates weren't available to make the calculation. -->
    <div v-if="displayRate" class="flexVCent clrP clrBr clrSh3 contentBox padSm toolTipNoWrap" :data-tip="tip">
      {{ ob.crypto.cryptoIcon({ className: 'margRSm', code: coinType, }) }}
      <strong class="tx5">{{ displayRate }}</strong>
    </div>

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
      displayRate: null,
    };
  },
  created () {
    this.initEventChain();

    this.loadData();
  },
  mounted () {
  },
  computed: {
    displayCur () {
      return app.settings.get('localCurrency') || 'USD';
    },
  },
  methods: {
    loadData () {
      this.listenTo(currencyEvents, 'exchange-rate-change', e => {
        if (e.changed.includes(this.displayCur) ||
          e.changed.includes(this.coinType)) {
          this.calcDisplayRate();
        }
      });

      this.calcDisplayRate ();
    },

    calcDisplayRate () {
      if (this.coinType === this.displayCur) {
        this.displayRate = null;
      }

      let rate = null;

      try {
        rate = convertAndFormatCurrency(1, this.coinType, this.displayCur, { skipConvertOnError: false });
      } catch (e) {
        // pass
      }

      this.displayRate = rate;
    },
  }
}
</script>
<style lang="scss" scoped></style>
