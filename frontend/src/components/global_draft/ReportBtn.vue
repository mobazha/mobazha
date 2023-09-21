<template>
  <div class="reportBtn">
    <div class="reportBtnShell toolTipNoWrap toolTipTop" @click="onClickReportBtn" :data-tip="tipText">
      <button :class="`iconBtnTn clrP clrBr tx2 ${ob.reported ? 'reported' : ''}`">
        <i :class="`ion-ios-flag ${ob.reported ? 'clrTErr' : ''}`"></i>
      </button>
    </div>
  </div>
</template>

<script>
import loadTemplate from '../../../backbone/utils/loadTemplate';
import baseVw from '../baseVw';
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
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted() {
    this.render();
  },
  computed: {
    params() {
      return {
        ...this.getState(),
      };
    },
    tipText() {
      return ob.reported ? ob.polyT('listingReport.btnTipReported') : ob.polyT('listingReport.btnTip');
    },
  },
  methods: {
    loadData(options = {}) {
      this.setState(options.initialState || {});

      this._state = {
        reported: false,
        ...(options.initialState || {}),
      };
    },

    className() {
      return 'reportBtn';
    },

    attributes() {
      // make it possible to tab to this element
      return { tabIndex: 0 };
    },

    onClickReportBtn(e) {
      e.stopPropagation();
      if (!this.getState().reported) {
        this.trigger('startReport');
        recordEvent('ReportListing');
      }
    },

    render() {
      loadTemplate('components/reportBtn.html', (t) => {
        this.$el.html(
          t({
            ...this.getState(),
          })
        );
      });

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
