<template>
  <div class="orderCompleteEvent rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.orderComplete.heading') }}</h2>
    <span class="clrT2 tx5b">{{ moment(ob.timestamp).format('lll') }}</span>
    <div class="border clrBr padMd">
      <div class="flex gutterHLg">
        <div class="col9">
          <div class="txB tx5 flexNoShrink rowTn">{{ ob.polyT('orderDetail.summaryTab.orderComplete.reviewLabel', { name: ob.buyerName }) }}</div>
          <div class="tx5" v-html="ob.parseEmojis(ob.review)"></div>
        </div>
        <div class="col3 ratingsCol">
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.overall') }}</div>
            <RatingsStrip :rating="rating.overall" />
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.quality') }}</div>
            <RatingsStrip :rating="rating.quality" />
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.asAdvertised') }}</div>
            <RatingsStrip :rating="rating.description" />
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.delivery') }}</div>
            <RatingsStrip :rating="rating.deliverySpeed" />
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.service') }}</div>
            <RatingsStrip :rating="rating.customerService" />
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import moment from 'moment';
import RatingsStrip from '../../../RatingsStrip.vue';

export default {
  components: {
    RatingsStrip,
  },
  props: {
    dataObject: {
      type: Object,
      default: {
        timestamp: undefined,
      },
    },
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.rating,
        timestamp: this.dataObject.timestamp,
      };
    },

    rating() {
      return this.dataObject.ratings[0];
    },
  },
  methods: {
    moment,
  }
}
</script>
<style lang="scss" scoped></style>
