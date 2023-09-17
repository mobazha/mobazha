<template>
  <div class="transactionFetchState">
    <div v-if="ob.isFetching">
      <div :class="`${ob.transactionsPresent ? 'txCtr' : 'center'}  padLg`">
        <SpinnerSVG :className="spinnerMd" />
      </div>
    </div>

    <div v-else-if="ob.fetchFailed">
      <div :class="`${ob.transactionsPresent ? '' : 'center'} txCtr tx5`">
        <div :class="`txB ${ob.initialFetchErrorMessage ? 'rowTn' : 'row'}`">{{ ob.polyT('wallet.transactions.fetchFailedMsg') }}</div>
        <div v-if="ob.fetchErrorMessage" class="row">{{ ob.fetchErrorMessage }}</div>
        <a class="btn clrP clrBr clrSh2 " @click="onClickRetryFetch">Retry</a>
      </div>
    </div>

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
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.setState(options.initialState || {});
    },

    onClickRetryFetch () {
      this.$emit('clickRetryFetch');
    },
  }
}
</script>
<style lang="scss" scoped></style>
