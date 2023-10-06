<template>
  <div class="completeOrderForm rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.completeOrderForm.heading') }}</h2>
    <div class="border clrBr padMd">
      <div class="flex gutterHLg">
        <div class="col9">
          <div class="flexVBase rowSm">
            <label class="txB tx5 required flexNoShrink" for="completeOrderReview">{{ ob.polyT('orderDetail.summaryTab.completeOrderForm.reviewLabel') }}</label>
            <div class="flexHRight">
              <span class="clrT2 tx6">{{ ob.polyT('orderDetail.summaryTab.completeOrderForm.maxReviewChars', { max: ob.constraints.maxReviewCharacters}) }}</span>
            </div>
          </div>
          <FormError v-if="ob.errors.review" :errors="ob.errors.review" />
          <textarea rows="8" name="review" class="clrBr clrP clrSh2 rowMd" id="completeOrderReview"
            placeholder="Write your review here…" :maxlength="ob.constraints.maxReviewCharacters" v-model="rating.review" />
          <div class="flexVCent gutterH">
            <ProcessingButton
              :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph js-completeOrder ${ob.isCompleting ? 'processing' : ''}`"
              :btnText="ob.polyT('orderDetail.summaryTab.completeOrderForm.btnCompleteOrder')"
              @click="onClickCompleteOrder" />
            <div class="gutterHSm">
              <FormError v-if="ob.errors.anonymous" :errors="ob.errors.anonymous" />
              <input type="checkbox" name="anonymous" id="completeOrderAnon" class="centerLabel" data-var-type="boolean"
                :checked="!rating.anonymous">
              <label for="completeOrderAnon" class="clrT2 tx5b">{{ ob.polyT('orderDetail.summaryTab.completeOrderForm.anonCheckLabel') }}</label>
            </div>
          </div>
        </div>
        <div class="col3 ratingsCol">
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.overall') }}</div>
            <FormError v-if="ob.errors.overall" :errors="ob.errors.overall" />
            <div class="ratingsContainer" data-rating-type="overall"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.quality') }}</div>
            <FormError v-if="ob.errors.quality" :errors="ob.errors.quality" />
            <div class="ratingsContainer" data-rating-type="quality"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.asAdvertised') }}</div>
            <FormError v-if="ob.errors.description" :errors="ob.errors.description" />
            <div class="ratingsContainer" data-rating-type="description"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.delivery') }}</div>
            <FormError v-if="ob.errors.deliverySpeed" :errors="ob.errors.deliverySpeed" />
            <div class="ratingsContainer" data-rating-type="deliverySpeed"></div>
          </div>
          <div class="row">
            <div class="txB tx5">{{ ob.polyT('ratingLabels.service') }}</div>
            <FormError v-if="ob.errors.customerService" :errors="ob.errors.customerService" />
            <div class="ratingsContainer" data-rating-type="customerService"></div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import {
  completeOrder,
  completingOrder,
  events as orderEvents,
} from '../../../../../backbone/utils/order';
import { recordEvent } from '../../../../../backbone/utils/metrics';
import Rating from '../../../../../backbone/models/order/orderCompletion/Rating';
import RatingsStrip from '../../../../../backbone/views/RatingsStrip';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.rating.toJSON(),
        errors: this.rating.validationError || {},
        isCompleting: !!completingOrder(this.model.id),
        constraints: this.rating.constraints || {},
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (!this.model) {
        throw new Error('Please provide an OrderCompletion model.');
      }

      if (!options.slug) {
        throw new Error('Please provide the listing slug.');
      }

      this.ratingStrips = {};
      this.slug = options.slug;

      const ratings = this.model.get('ratings');

      if (ratings.length) {
        this.rating = ratings.at(0);
      } else {
        this.rating = new Rating();
        ratings.push(this.rating);
      }

      this.listenTo(orderEvents, 'completingOrder', () => {
        this.getCachedEl('.js-completeOrder').addClass('processing');
      });

      this.listenTo(orderEvents, 'completeOrderComplete completeOrderFail', () => {
        this.getCachedEl('.js-completeOrder').removeClass('processing');
      });
    },

    onClickCompleteOrder () {
      const formData = this.getFormData();

      const data = {
        ...formData,
        anonymous: !formData.anonymous,
        // If a rating is not set, the RatingStrip view will return 0. We'll
        // send undefined in that case since it gives us the error message we
        // prefer.
        overall: this.ratingStrips.overall.rating || undefined,
        quality: this.ratingStrips.quality.rating || undefined,
        description: this.ratingStrips.description.rating || undefined,
        deliverySpeed: this.ratingStrips.deliverySpeed.rating || undefined,
        customerService: this.ratingStrips.customerService.rating || undefined,
        slug: this.slug,
      };

      this.rating.set(data);
      this.rating.set(data, { validate: true });

      if (!this.rating.validationError) {
        completeOrder(this.model.id, this.model.toJSON());
        recordEvent('OrderDetails_CompleteOrder');
      }

      this.render();
      const $firstErr = $('.errorList:first');
      if ($firstErr.length) $firstErr[0].scrollIntoViewIfNeeded();
    },

    render () {
      this.isCompleting = !!completingOrder(this.model.id);

      $('.ratingsContainer').each((index, element) => {
        const $el = $(element);
        const type = $el.data('ratingType');

        if (!type) {
          throw new Error('Unable to render a ratings strips because it\'s container does not ' +
            'specify a type.');
        }

        if (this.ratingStrips[type]) this.ratingStrips[type].remove();
        this.ratingStrips[type] = this.createChild(RatingsStrip, {
          initialState: {
            curRating: this.rating.get(type) || 0,
            clickable: true,
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
