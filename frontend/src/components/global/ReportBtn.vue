<template>
  <div class="reportBtn">

    <div class="reportBtnShell toolTipNoWrap  toolTipTop" @click="onClickReportBtn" :data-tip="tipText">
      <button :class="`iconBtnTn clrP clrBr tx2 ${ob.reported ? 'reported' : ''}`">
        <i :class="`ion-ios-flag ${ob.reported ? 'clrTErr' :''}`"></i>
      </button>
    </div>

  </div>
</template>

<script setup>
import loadTemplate from '../../../backbone/utils/loadTemplate';
import { recordEvent } from '../../../backbone/utils/metrics';


const props = defineProps({
  phase: String,
})

const tipText = ob.reported ? ob.polyT('listingReport.btnTipReported') : ob.polyT('listingReport.btnTip');

loadData(props);

render();

function loadData (options = {}) {
  super(options);

  this._state = {
    reported: false,
    ...options.initialState || {},
  };
}

function className () {
  return 'reportBtn';
}

function attributes () {
  // make it possible to tab to this element
  return { tabIndex: 0 };
}

function onClickReportBtn (e) {
  e.stopPropagation();
  if (!this.getState().reported) {
    this.trigger('startReport');
    recordEvent('ReportListing');
  }
}


function render () {
  loadTemplate('components/reportBtn.html', (t) => {
    this.$el.html(t({
      ...this.getState(),
    }));
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
