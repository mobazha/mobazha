<template>
<div class="moderatorsStatus flexCent gutterHTn tx6 clrBr clrP <% if(ob.hidden) print('hide') %>">
  <SpinnerSVG v-if="ob.showSpinner && ob.loading" className="spinnerTxt js-spinner" />
  <span class="clrT4">{{ statusInfo }}</span>

  <button v-if="ob.showLoadBtn" class="btnAsLink tx6 clrT2 browseMore js-browseMore" :disabled="ob.showSpinner">
    {{ ob.polyT('moderators.browseMoreModerators') }}
  </button>
</div>
</template>

<script setup>
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

</script>
<style lang="scss" scoped>
</style>
