<template>
  <ul class="unstyled row">
    <li class="flexVCent gutterHSm clrBr" v-for="(cur, j) in ob.processedCurs" :key="j">
      <span class="ion-ios-checkmark-empty clrTEm tx2"></span>
      <CryptoIcon :code="cur.code" />
      <div class="acceptedCurName">{{ cur.displayName }}</div>
    </li>
  </ul>
</template>

<script>
import _ from 'underscore';
import { ensureMainnetCode } from '../../../backbone/data/walletCurrencies';
import app from '../../../backbone/app';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      _state: {
        currencies: [],
        processedCurs: [],
        sort: true,
      }
    };
  },
  created() {
    this.loadData(this.options);
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        ...options,
        initialState: {
          currencies: [],
          processedCurs: [],
          sort: true,
          ...options.initialState,
        },
      };

      this.baseInit(opts);
    },

    setState(state = {}) {
      const curState = this.getState();
      const processedState = {
        ...state,
        // This is a derived field and should not be directly set
        processedCurs: Array.isArray(curState.processedCurs) ? curState.processedCurs : [],
      };

      if (Array.isArray(processedState.currencies) && (curState.sort !== processedState.sort || !_.isEqual(curState.currencies, state.currencies))) {
        processedState.processedCurs = processedState.currencies.map((cur) => {
          const code = ensureMainnetCode(cur);
          const displayName = app.polyglot.t(`cryptoCurrencies.${code}`, { _: cur });

          return {
            code,
            displayName,
            sortDisplayName: displayName.toUpperCase(),
          };
        });

        if (processedState.sort) {
          processedState.processedCurs = _.sortBy(processedState.processedCurs, 'sortDisplayName').map((cur) => {
            delete cur.sortDisplayName;
            return cur;
          });
        }
      }

      _.extend(this._state, processedState);
    },
  },
};
</script>
<style lang="scss" scoped></style>
