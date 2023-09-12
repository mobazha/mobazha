<template>
  <div class="disputeAcceptanceEvent rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.disputeAcceptance.heading') }}</h2>
    <div v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ ob.moment(ob.timestamp).format('lll') }}</span>
    </div>
    <div class="border clrBr padMd">
      <div class="flexVCent gutterH clrT">
        <div class="avatarCol disc clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.closerAvatarHashes)"></div>
        <div class="flexExpand tx5">
          <div class="rowTn txB">{{ introLine }}</div>
          <div>{{ subText }}</div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import _ from 'underscore';
import moment from 'moment';

export default {
  mixins: [],
  props: {
    cart: Object,
  },
  data () {
    return {
      closerName: '',
      closerAvatarHashes: {},
      buyerViewing: false,
      vendorProcessingError: false,

      introLine: '',
      subText: '',
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
  },
  computed: {
    introLine () {
      if (this.closerName) {
        return ob.polyT('orderDetail.summaryTab.disputeAcceptance.userAcceptedPayout', { name: this.closerName, });
      } else {
        return this.acceptedByBuyer ?
          ob.polyT('orderDetail.summaryTab.disputeAcceptance.genericBuyerAcceptedPayout') :
          ob.polyT('orderDetail.summaryTab.disputeAcceptance.genericVendorAcceptedPayout');
      }
    },

    subText () {
      if (!this.vendorProcessingError) {
        // Since the text indicates the order will be complete after leaving a review and you
        // can't leave a review if the vendor has an error processing the order, we'll omit the
        // text in that case.
        return this.buyerViewing ?
          ob.polyT('orderDetail.summaryTab.disputeAcceptance.orderCompleteWhenYouReview') :
          ob.polyT('orderDetail.summaryTab.disputeAcceptance.orderCompleteWhenBuyerReviews');
      }
      return '';
    },
  },
  methods: {
    moment,
    loadData (options = {}) {

    },

  }
}
</script>
<style lang="scss" scoped></style>
