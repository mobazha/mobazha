<template>
  <ul class="unstyled row">
    <div v-for="(cur, j) in ob.processedCurs" :key="j">
      <li class="flexVCent gutterHSm clrBr">
        <span class="ion-ios-checkmark-empty clrTEm tx2"></span>
        {{ ob.crypto.cryptoIcon({ code: cur.code }) }}
        <div class="acceptedCurName">{{ cur.displayName }}</div>
      </li>
    </div>

  </ul>
</template>

<script setup>
import _ from 'underscore';
import loadTemplate from '../../../backbone/utils/loadTemplate';
import { ensureMainnetCode } from '../../../backbone/data/walletCurrencies';
import app from '../../../backbone/app';


const props = defineProps({
  phase: String,
  outdatedHash: String,
})

loadData(props);

render();

function loadData (options = {}) {
  const opts = {
    ...options,
    initialState: {
      currencies: [],
      processedCurs: [],
      sort: true,
      ...options.initialState,
    },
  };
}

function setState (state = {}, options = {}) {
  const curState = this.getState();
  const processedState = {
    ...state,
    // This is a derived field and should not be directly set
    processedCurs: Array.isArray(curState.processedCurs) ?
      curState.processedCurs : [],
  };

  if (Array.isArray(processedState.currencies) &&
    (
      curState.sort !== processedState.sort ||
      !_.isEqual(curState.currencies, state.currencies)
    )) {
    processedState.processedCurs = processedState.currencies
      .map(cur => {
        const code = ensureMainnetCode(cur);
        const displayName = app.polyglot.t(`cryptoCurrencies.${code}`, { _: cur });

        return {
          code,
          displayName,
          sortDisplayName: displayName.toUpperCase(),
        };
      });

    if (processedState.sort) {
      processedState.processedCurs = _.sortBy(processedState.processedCurs, 'sortDisplayName')
        .map(cur => {
          delete cur.sortDisplayName;
          return cur;
        });
    }
  }
}

function render () {
  loadTemplate('components/supportedCurrenciesList.html', (t) => {
    this.$el.html(t({
      ...this.getState(),
    }));
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
