<template>
  <div class="transactionFetchState">
    <template v-if="ob.isFetching">
      <div :class="`${ob.transactionsPresent ? 'txCtr' : 'center'}  padLg`">
        <SpinnerSVG className="spinnerMd" />
      </div>
    </template>

    <template v-else-if="ob.fetchFailed">
      <div :class="`${ob.transactionsPresent ? '' : 'center'} txCtr tx5`">
        <div :class="`txB ${ob.initialFetchErrorMessage ? 'rowTn' : 'row'}`">{{ ob.polyT('wallet.transactions.fetchFailedMsg') }}</div>
        <div v-if="ob.fetchErrorMessage" class="row">{{ ob.fetchErrorMessage }}</div>
        <a class="btn clrP clrBr clrSh2" @click="onClickRetryFetch">Retry</a>
      </div>
    </template>
  </div>
</template>

<script>
export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {};
  },
  created() {
    this.initEventChain();
  },
  watch: {
    options: {
      handler(val) {
        this.loadData(val);
      },
      deep: true,
      immediate: true,
    },
  },
  mounted() {},
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
      this.setState(options.initialState || {});
    },

    onClickRetryFetch() {
      this.$emit('clickRetryFetch');
    },
  },
};
</script>
<style lang="scss" scoped></style>
