<template>
  <div class="cryptoTicker">
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

<script>
import app from '../../../backbone/app';
import {
  convertAndFormatCurrency,
  events as currencyEvents,
} from '../../../backbone/utils/currency';
import loadTemplate from '../../../backbone/utils/loadTemplate';


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
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
    params () {
      return {
        ...this.getState(),
      };
    }
  },
	methods: {
  loadData(options = {}) {
    const opts = {
      ...options,
      initialState: {
        displayCur: app.settings.get('localCurrency') || 'USD',
        ...options.initialState,
      },
    };

    super(opts);
    this.listenTo(app.settings, 'change:localCurrency',
      (md, cur) => this.setState({ displayCur: cur }));
    this.listenTo(currencyEvents, 'exchange-rate-change', e => {
      if (e.changed.includes(this.getState().displayCur) ||
        e.changed.includes(this.getState().coinType)) {
        this.setState({ displayRate: this.calcDisplayRate() });
      }
    });
  },

  setState(state, options) {
    const curState = this.getState();

    const mergedState = {
      ...curState,
      ...state,
    };

    if (typeof mergedState.coinType !== 'string' || !mergedState.coinType) {
      throw new Error('The state must include a coinType.');
    }

    if (mergedState.coinType === mergedState.displayCur) {
      mergedState.displayRate = null;
    } else if (curState.coinType !== mergedState.coinType ||
      curState.displayCur !== mergedState.displayCur) {
      mergedState.displayRate =
        this.calcDisplayRate(mergedState.coinType, mergedState.displayCur);
    }

    return super.setState(mergedState, options);
  },

  calcDisplayRate(
    coinType = this.getState().coinType,
    displayCur = this.getState().displayCur
  ) {
    let rate = null;

    try {
      rate = convertAndFormatCurrency(1, coinType, displayCur, { skipConvertOnError: false });
    } catch (e) {
      // pass
    }

    return rate;
  },

  render() {
    loadTemplate('components/cryptoTicker.html', (t) => {
      this.$el.html(t({
        ...this.getState(),
      }));
    });

    return this;
  }

  }
}
</script>
<style lang="scss" scoped>
</style>
