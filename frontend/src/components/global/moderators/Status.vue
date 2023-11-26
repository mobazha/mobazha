<template>
  <div class="moderatorsStatus flexCent gutterHTn tx6 clrBr clrP">
    <SpinnerSVG v-if="ob.showSpinner && ob.loading" className="spinnerTxt js-spinner" />
    <span class="clrT4">{{ statusInfo }}</span>

    <button v-if="ob.showLoadBtn" class="btnAsLink tx6 clrT2 browseMore" @click="clickBrowseMore" :disabled="ob.showSpinner">
      {{ ob.polyT('moderators.browseMoreModerators') }}
    </button>
  </div>
</template>

<script>
import _ from 'underscore';

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
        showSpinner: true,
        showLoadBtn: false,
        loaded: 0,
        toLoad: 0,
        total: 0,
        mode: 'loaded',
        loading: false,
      }
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {},
  unmounted() {
    clearTimeout(this.spinnerTimeout);
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
    statusInfo() {
      const ob = this.ob;

      let statusInfo = ob.polyT('moderators.moderatorsLoading');
      if (ob.mode === 'loaded') {
        statusInfo = ob.polyT('moderators.moderatorsLoaded', { total: ob.total, smart_count: ob.total });
      } else if (ob.mode === 'loadingXofY') {
        statusInfo = ob.polyT('moderators.moderatorsXofY', { current: ob.loaded + 1, total: ob.toLoad });
      } else if (ob.mode === 'loadingXofYTimedOut') {
        const remainder = ob.toLoad - ob.loaded;
        statusInfo = ob.polyT('moderators.loadingXofYTimedOut', { loaded: ob.loaded, total: ob.toLoad, remainder, smart_count: remainder });
      }
      return statusInfo;
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        ...options,
        initialState: {
          showSpinner: true,
          showLoadBtn: false,
          loaded: 0,
          toLoad: 0,
          total: 0,
          mode: 'loaded',
          loading: false,
          ...(options.initialState || {}),
        },
      };

      this.baseInit(opts);
    },

    setState(state = {}) {
      const combinedState = { ...this.getState(), ...state };
      // Any time the state is set to loading, set the spinner timer if needed.
      if (state.loading && combinedState.showSpinner) {
        clearTimeout(this.spinnerTimeout);
        this.spinnerTimeout = setTimeout(() => {
          let mode = this.getState().mode;
          if (mode === 'loadingXofY') mode = 'loadingXofYTimedOut';
          this.setState({ showSpinner: false, mode });
        }, 10000);
      }
      _.extend(this._state, state);
    },

    clickBrowseMore() {
      this.$emit('browseMore');
    },
  },
};
</script>
<style lang="scss" scoped></style>
