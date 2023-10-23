<template>
  <div class="reportBtn">
    <div class="reportBtnShell toolTipNoWrap toolTipTop" @click.stop="onClickReportBtn" :data-tip="tipText">
      <button :class="`iconBtnTn clrP clrBr tx2 ${ob.reported ? 'reported' : ''}`">
        <i :class="`ion-ios-flag ${ob.reported ? 'clrTErr' : ''}`"></i>
      </button>
    </div>
  </div>
</template>

<script>
import { recordEvent } from '../../../backbone/utils/metrics';

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
    this.loadData(this.options);
  },
  mounted() {
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
    tipText() {
      let ob = this.ob;
      return ob.reported ? ob.polyT('listingReport.btnTipReported') : ob.polyT('listingReport.btnTip');
    },
  },
  methods: {
    loadData(options = {}) {
      this.baseInit(options);

      this._state = {
        reported: false,
        ...(options.initialState || {}),
      };
    },

    attributes() {
      // make it possible to tab to this element
      return { tabIndex: 0 };
    },

    onClickReportBtn(e) {
      if (!this.getState().reported) {
        this.$emit('startReport');
        recordEvent('ReportListing');
      }
    },
  },
};
</script>
<style lang="scss" scoped></style>
