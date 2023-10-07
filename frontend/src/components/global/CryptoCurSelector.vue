<template>
  <ul class="unstyled borderStackedAll curSelector">
    <li class="clrBr curRow" v-for="(cur, j) in ob.processedCurs" :key="j">
      <span class="curControlWrapper gutterHSm" :disabled="cur.disabled" @click="handleCurClick(cur.code)">
        <input
          :type="ob.controlType"
          :id="`curSel${cur.code}${ob.cid}`"
          class="centerLabel"
          :name="ob.controlType === 'radio' ? 'currencies' : ''"
          :checked="cur.active && !cur.disabled"
        />
        <label :for="`curSel${cur.code}${ob.cid}`">
          <CryptoIcon :code="cur.code" />
          <span class="curName noOverflow">{{ cur.displayName }}</span>
        </label>
      </span>
      <template v-if="cur.disabled && ob.disabledMsg">
        <span class="disabledMsg noOverflow clrTErr tx5b">{{ ob.disabledMsg }}</span>
      </template>
    </li>
  </ul>
</template>

<script>
import _ from 'underscore';
import app from '../../../backbone/app';
import { isSupportedWalletCur } from '../../../backbone/data/walletCurrencies';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      _options: {
        disabledMsg: '',
      },
      _state: {
        controlType: 'checkbox',
        currencies: [],
        activeCurs: [],
        disabledCurs,
        sort: false,
      }
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        cid: this.cid,
        ...this._options,
        ...this._state,
      };
    }
  },
  methods: {
    loadData(options = {}) {
      let disabledCurs = [];

      if (
        Array.isArray(options.initialState.disabledCurs) &&
        Array.isArray(options.initialState.currencies)
      ) {
        disabledCurs =
          options.initialState.currencies.filter(c => !isSupportedWalletCur(c));
      }

      const opts = {
        disabledMsg: '',
        ...options,
        initialState: {
          controlType: 'checkbox',
          currencies: [],
          activeCurs: [],
          disabledCurs,
          sort: false,
          ...options.initialState,
        },
      };
      this.baseInit(opts);
      this._options = opts;
    },

    handleCurClick(code) {
      let activeCurs = [...this.getState().activeCurs];
      // Toggle the current active state when clicked.
      const nowActive = !activeCurs.includes(code);

      if (this.getState().controlType === 'radio') {
        activeCurs = [code];
      } else {
        if (nowActive) activeCurs.push(code);
        else activeCurs = activeCurs.filter((c) => c !== code);
      }

      this.$emit('currencyClicked', {
        currency: code,
        active: nowActive,
        activeCurs,
      });

      this.setState({ activeCurs });
    },

    setState(state = {}, options = {}) {
      const controlTypes = ['checkbox', 'radio'];
      const curState = this.getState();

      if (state.hasOwnProperty('controlType') && !controlTypes.includes(state.controlType)) {
        throw new Error('If provided the controlType must be a valid value.');
      }

      const checkCurArray = (fieldName) => {
        if (state.hasOwnProperty(fieldName) && !Array.isArray(state[fieldName])) {
          throw new Error(`If provided the ${fieldName} must be an array.`);
        }
      };

      ['currencies', 'activeCurs', 'disabledCurs'].forEach((field) => checkCurArray(field));

      // This is a derived field and should not be directly set
      delete state.processedCurs;

      const processedState = {
        ...curState,
        ...state,
        currencies: Array.isArray(state.currencies) ? [...new Set(state.currencies)] : curState.currencies,
      };

      // Radio controls must have no more than one active currency.
      if (processedState.controlType === 'radio') {
        processedState.activeCurs = processedState.activeCurs && processedState.activeCurs.length ? [processedState.activeCurs[0]] : [];
      }

      // Remove any disabled currencies from the active list.
      if (state.activeCurs || state.disabledCurs) {
        processedState.activeCurs = [...new Set(processedState.activeCurs.filter((c) => !processedState.disabledCurs.includes(c)))];
      }

      // If necessary, create the processed curs
      if (!processedState.processedCurs || state.currencies || !!processedState.sort !== !!state.sort) {
        processedState.processedCurs = processedState.currencies.map((cur) => ({
          code: cur,
          displayName: app.polyglot.t(`cryptoCurrencies.${cur}`, {
            _: cur,
          }),
          disabled: processedState.disabledCurs.includes(cur),
          active: processedState.activeCurs.includes(cur),
        }));

        if (processedState.sort) {
          const locale = app.localSettings.standardizedTranslatedLang() || 'en-US';
          processedState.processedCurs.sort((a, b) => a.displayName.localeCompare(b.displayName, locale, { sensitivity: 'base' }));
        }
      } else if (state.activeCurs || state.disabledCurs) {
        // If active or disabled lists are passed in, we'll assume they're
        // different and ensure the processedCurrencies list reflects them.
        processedState.processedCurs = processedState.processedCurs.map((cur) => ({
          ...cur,
          active: processedState.activeCurs.includes(cur.code),
          disabled: processedState.disabledCurs.includes(cur.code),
        }));
      }

      _.extend(this._state, processedState);
    },
  },
};
</script>
<style lang="scss" scoped></style>
