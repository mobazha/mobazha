<template>
  <div>
    <div class="flexRow gutterH">
      <div class="col4">
        <label>{{ ob.polyT('editListing.coupons.titleLabel') }}</label>
      </div>
      <div class="col4">
        <label class="required">{{ ob.polyT('editListing.coupons.couponCodeLabel') }}</label>
      </div>
      <div class="col4">
        <label class="required">{{ ob.polyT('editListing.coupons.discountLabel') }}</label>
      </div>
    </div>
    <div class="js-couponsWrap padKids padStack padTop0">
      <template v-for="coupon in collection" :key="coupon.cid">
        <Coupon ref="couponViews" :bb="function() {
            return {
              model: coupon,
            }
          }"
          @remove-click="onRemoveCouponView" />
      </template>
    </div>
    <a class="clrBr clrP clrTEm" v-show="collection.length < ob.maxCouponCount" @click="onClickAddCoupon">{{
      ob.polyT('editListing.coupons.btnAddCoupon') }}</a>
  </div>
</template>

<script>
import CouponMd from '../../../../backbone/models/listing/Coupon';
import Coupon from './Coupon.vue';


export default {
  components: {
    Coupon,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        coupons: this.collection.toJSON(),
        maxCouponCount: this.options.maxCouponCount,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.collection) {
        throw new Error('Please provide a collection.');
      }

      if (typeof options.maxCouponCount === 'undefined') {
        throw new Error('Please provide the maximum coupon count.');
      }
    },

    onClickAddCoupon () {
      this.collection.add(new CouponMd());

      this.$nextTick(() => {
        if (this.collection.length) (this.$refs.couponViews[this.collection.length - 1]).setFocus();
      });
    },

    setCollectionData () {
      (this.$refs.couponViews ?? []).forEach(coupon => coupon.setModelData());
    },

    onRemoveCouponView(md) {
      this.collection.remove(md)
    },
  }
}
</script>
<style lang="scss" scoped></style>
