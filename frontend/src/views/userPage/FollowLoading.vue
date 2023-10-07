<template>
  <div class="followLoadingState txCtr tx5">
    <template v-if="ob.isFetching">
      <div class="loadingSpinnerWrap">
        <SpinnerSVG className="spinnerMd" />
      </div>
    </template>

    <template v-else-if="ob.fetchFailed">
      <p>{{ ob.fetchErrorTitle }}</p>
      <template v-if="ob.fetchErrorMsg">
        <p>{{ ob.fetchErrorMsg }}</p>
      </template>
      <button class="btn normalBtn clrP clrBr" @click="onClickRetry">{{ ob.polyT('userPage.followTab.btnRetry') }}</button>
    </template>

    <template v-else-if="ob.noResults">
      <p>{{ ob.noResultsMsg }}</p>
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
    return {
      _state: {
        isFetching: false,
        noResults: false,
        noResultsMsg: '',
        fetchFailed: false,
        fetchErrorTitle: '',
        fetchErrorMsg: '',
      }
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
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
      const opts = {
        initialState: {
          isFetching: false,
          noResults: false,
          noResultsMsg: '',
          fetchFailed: false,
          fetchErrorTitle: '',
          fetchErrorMsg: '',
          ...(options.initialState || {}),
        },
        ...options,
      };

      this.baseInit(opts);
    },

    onClickRetry() {
      this.$emit('retry-click');
    },
  },
};
</script>
<style lang="scss" scoped></style>
