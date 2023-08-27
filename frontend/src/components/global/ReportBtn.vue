<template>

  <div class="reportBtnShell toolTipNoWrap  toolTipTop js-reportBtn"  :data-tip="tipText">
    <button :class="`iconBtnTn clrP clrBr tx2 ${ob.reported ? 'reported' : ''}`">
      <i :class="`ion-ios-flag ${ob.reported ? 'clrTErr' :''}`"></i>
    </button>
  </div>

</template>

<script setup>
const props = defineProps({
  phase: String,
})

const tipText = ob.reported ? ob.polyT('listingReport.btnTipReported') : ob.polyT('listingReport.btnTip');
</script>
<style lang="scss" scoped>
</style>