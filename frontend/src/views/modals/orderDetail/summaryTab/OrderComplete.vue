<template>
  <div class="orderCompleteEvent rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.orderComplete.heading') }}</h2>
    <span class="clrT2 tx5b">{{ ob.moment(dataObject.timestamp).format('lll') }}</span>
    <div class="border clrBr padMd">
      <div class="flex gutterHLg">
        <div class="col9">
          <div class="txB tx5 flexNoShrink rowTn">{{ ob.polyT('orderDetail.summaryTab.orderComplete.reviewLabel', { name: ob.buyerName }) }}</div>
          <div class="tx5">{{ ob.parseEmojis(ob.review) }}</div>
        </div>
        <div class="col3 ratingsCol">
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.overall') }}</div>
            <div class="ratingsContainer" data-rating-type="overall"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.quality') }}</div>
            <div class="ratingsContainer" data-rating-type="quality"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.asAdvertised') }}</div>
            <div class="ratingsContainer" data-rating-type="description"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.delivery') }}</div>
            <div class="ratingsContainer" data-rating-type="deliverySpeed"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.service') }}</div>
            <div class="ratingsContainer" data-rating-type="customerService"></div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import moment from 'moment';
import RatingsStrip from '../../../../../backbone/views/RatingsStrip';


export default {
  mixins: [],
  props: {
    cart: Object,
  },
  data () {
    return {
      ob: {},
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
  },
  methods: {
    moment,

    loadData (options = {}) {
      super(options);

      if (!options.dataObject) {
        throw new Error('Please provide a buyerOrderCompletion data object.');
      }

      this.dataObject = options.dataObject;
      this.ratingStrips = {};
    },

    render () {
      const rating = this.dataObject.ratings[0];
      $('.ratingsContainer').each((index, element) => {
        const $el = $(element);
        const type = $el.data('ratingType');

        if (!type) {
          throw new Error('Unable to render the ratings strips because it\'s container does not ' +
            'specify a type.');
        }

        if (this.ratingStrips[type]) this.ratingStrips[type].remove();
        this.ratingStrips[type] = this.createChild(RatingsStrip, {
          initialState: {
            curRating: rating[type] || 0,
          },
        });

        $el.append(this.ratingStrips[type].render().el);
      });

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
