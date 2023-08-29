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

<script setup>
import app from '../../../backbone/app';
import {
  convertAndFormatCurrency,
  events as currencyEvents,
} from '../../../backbone/utils/currency';
import loadTemplate from '../../../backbone/utils/loadTemplate';

const props = defineProps({
  phase: String,
  outdatedHash: String,
})

const tip = ob.polyT('cryptoTicker.currentPrice', {
  cur: ob.polyT(`cryptoCurrencies.${ob.coinType}`, { _: ob.coinType }),
});

loadData(props);

render();

function loadData (options = {}) {
  const opts = {
    ...options,
    initialState: {
      displayCur: app.settings.get('localCurrency') || 'USD',
      ...options.initialState,
    },
  };

  super(opts);
  this.listenTo(app.settings, 'change:localCurrency',
    (md, cur) => setState({ displayCur: cur }));
  this.listenTo(currencyEvents, 'exchange-rate-change', e => {
    if (e.changed.includes(this.getState().displayCur) ||
      e.changed.includes(this.getState().coinType)) {
      setState({ displayRate: this.calcDisplayRate() });
    }
  });
}

function setState (state, options) {
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
      calcDisplayRate(mergedState.coinType, mergedState.displayCur);
  }

  return super.setState(mergedState, options);
}

function calcDisplayRate (
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
}

function render () {
  loadTemplate('components/cryptoTicker.html', (t) => {
    this.$el.html(t({
      ...this.getState(),
    }));
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
