<template>
  <div class="moderatorStatus">
    <div class="moderatorsStatus flexCent gutterHTn tx6 clrBr clrP" v-show="!ob.hidden">
      <SpinnerSVG v-if="ob.showSpinner && ob.loading" className="spinnerTxt js-spinner" />
      <span class="clrT4">{{ statusInfo }}</span>

      <button v-if="ob.showLoadBtn" class="btnAsLink tx6 clrT2 browseMore" @click="clickBrowseMore" :disabled="ob.showSpinner">
        {{ ob.polyT('moderators.browseMoreModerators') }}
      </button>
    </div>

  </div>
</template>

<script setup>
import loadTemplate from '../../../../backbone/utils/loadTemplate';


const props = defineProps({
  feeLevel: String,
})

let statusInfo = ob.polyT('moderators.moderatorsLoading');
if (ob.mode === 'loaded') {
  statusInfo = ob.polyT('moderators.moderatorsLoaded', { total: ob.total, smart_count: ob.total });
} else if (ob.mode === 'loadingXofY') {
  statusInfo = ob.polyT('moderators.moderatorsXofY', { current: ob.loaded + 1, total: ob.toLoad });
} else if (ob.mode === 'loadingXofYTimedOut') {
  const remainder = ob.toLoad - ob.loaded;
  statusInfo = ob.polyT('moderators.loadingXofYTimedOut', { loaded: ob.loaded, total: ob.toLoad, remainder, smart_count: remainder });
}

loadData(props);

render();

function loadData (options = {}) {
  const opts = {
    ...options,
    initialState: {
      hidden: true,
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

  super(opts);
  this.options = opts;
}

function setState (state = {}, options = {}) {
  const combinedState = { ...this.getState(), ...state };
  // Any time the state is set to loading, set the spinner timer if needed.
  if (state.loading && combinedState.showSpinner) {
    clearTimeout(this.spinnerTimeout);
    this.spinnerTimeout = setTimeout(() => {
      let mode = this.getState().mode;
      if (mode === 'loadingXofY') mode = 'loadingXofYTimedOut';
      setState({ showSpinner: false, mode });
    }, 10000);
  }
  super.setState(state, options);
}

function clickBrowseMore () {
  this.trigger('browseMore');
}

function remove () {
  clearTimeout(this.spinnerTimeout);
  super.remove();
}

function render () {
  loadTemplate('components/moderators/status.html', (t) => {
    this.$el.html(t({
      ...this.getState(),
    }));
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
