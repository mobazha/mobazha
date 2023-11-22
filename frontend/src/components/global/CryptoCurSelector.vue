<template>
  <ul class="unstyled borderStackedAll curSelector">
    <li class="clrBr curRow" v-for="cur in processedState.processedCurs" :key="cur.code">
      <span class="curControlWrapper gutterHSm" :disabled="cur.disabled" @click="handleCurClick(cur.code)">
        <input
          :type="controlType"
          :id="`curSel${cur.code}`"
          class="centerLabel"
          :name="controlType === 'radio' ? 'currencies' : ''"
          :checked="cur.active && !cur.disabled"
        />
        <label :for="`curSel${cur.code}`">
          <CryptoIcon :code="cur.code" />
          <span class="curName noOverflow">{{ cur.displayName }}</span>
        </label>
      </span>
      <template v-if="cur.disabled && disabledMsg">
        <span class="disabledMsg noOverflow clrTErr tx5b">{{ disabledMsg }}</span>
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
      default: {
        controlType: 'checkbox',
        currencies: [],
        disabledCurs: [],
        sort: false,
        disabledMsg: '',
      },
    },
    activeCurs: Object,
  },
  emits: ['update:activeCurs', 'currencyClicked'],
  data() {
    return {
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    processedState() {
      const controlTypes = ['checkbox', 'radio'];

      if (!controlTypes.includes(this.controlType)) {
        throw new Error('If provided the controlType must be a valid value.');
      }

      const checkCurArray = (fieldName) => {
        if (_.has(this, fieldName) && !Array.isArray(this[fieldName])) {
          throw new Error(`If provided the ${fieldName} must be an array.`);
        }
      };
      ['currencies', 'activeCurs', 'disabledCurs'].forEach((field) => checkCurArray(field));

      const processedState = {
        activeCurs: this.activeCurs,
      };

      // Radio controls must have no more than one active currency.
      if (this.controlType === 'radio') {
        processedState.activeCurs = this.activeCurs && this.activeCurs.length ? [this.activeCurs[0]] : [];
      }

      // Remove any disabled currencies from the active list.
      if (processedState.activeCurs || this.disabledCurs) {
        processedState.activeCurs = [...new Set(processedState.activeCurs.filter((c) => !this.disabledCurs.includes(c)))];
      }
      processedState.activeCurs = _.uniq(processedState.activeCurs);

      // If necessary, create the processed curs
      processedState.processedCurs = _.uniq(this.currencies).map((cur) => ({
        code: cur,
        displayName: app.polyglot.t(`cryptoCurrencies.${cur}`, {
          _: cur,
        }),
        disabled: this.disabledCurs.includes(cur),
        active: processedState.activeCurs.includes(cur),
      }));

      if (this.sort) {
        const locale = app.localSettings.standardizedTranslatedLang() || 'en-US';
        processedState.processedCurs.sort((a, b) => a.displayName.localeCompare(b.displayName, locale, { sensitivity: 'base' }));
      }

      return processedState;
    },
  },
  methods: {
    loadData(options = {}) {
      let disabledCurs = [];

      if (Array.isArray(options.disabledCurs) && Array.isArray(options.currencies)) {
        disabledCurs = options.currencies.filter(c => !isSupportedWalletCur(c));
      }

      const opts = {
        disabledMsg: '',
        controlType: 'checkbox',
        currencies: [],
        disabledCurs,
        sort: false,

        ...options,
      };
      this.baseInit(opts);
    },

    handleCurClick(code) {
      let activeCurs = this.processedState.activeCurs;
      // Toggle the current active state when clicked.
      const nowActive = !activeCurs.includes(code);

      if (this.controlType === 'radio') {
        activeCurs = [code];
      } else {
        if (nowActive) activeCurs.push(code);
        else activeCurs = activeCurs.filter((c) => c !== code);
      }

      this.$emit('update:activeCurs', activeCurs);
      this.$emit('currencyClicked', { currency: code, active: nowActive, activeCurs, });
    },
  },
};
</script>
<style lang="scss" scoped></style>
