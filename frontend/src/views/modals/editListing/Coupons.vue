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
    <div class="js-couponsWrap padKids padStack padTop0"></div>
    <a class="clrBr clrP clrTEm" v-show="ob.coupons.length < ob.maxCouponCount" @click="onClickAddCoupon">{{
      ob.polyT('editListing.coupons.btnAddCoupon') }}</a>
  </div>
</template>

<script>
import CouponMd from '../../../../backbone/models/listing/Coupon';
import Coupon from './Coupon';


export default {
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
        coupons: this._collection,
        maxCouponCount: this.maxCouponCount,
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

      this.baseInit(options);

      this.couponViews = [];

      this.listenTo(this.collection, 'add', (md, cl) => {
        const index = cl.indexOf(md);
        const view = this.createCouponView(md);

        if (index) {
          this.$couponsWrap.find('> *')
            .eq(index - 1)
            .after(view.render().el);
        } else {
          this.$couponsWrap.prepend(view.render().el);
        }

        this.couponViews.splice(index, 0, view);

        if (this.collection.length >= this.options.maxCouponCount) {
          this.$addCoupon.addClass('hide');
        }
      });

      this.listenTo(this.collection, 'remove', (md, cl, removeOpts) => {
        (this.couponViews.splice(removeOpts.index, 1)[0]).remove();

        if (this.collection.length < this.options.maxCouponCount) {
          this.$addCoupon.removeClass('hide');
        }
      });
    },

    onClickAddCoupon () {
      this.collection.add(new CouponMd());
      this.couponViews[this.couponViews.length - 1]
        .$('input[name=title]')
        .focus();
    },

    setCollectionData () {
      this.couponViews.forEach(coupon => coupon.setModelData());
    },

    createCouponView (model, options = {}) {
      const view = this.createChild(Coupon, {
        model,
        ...options,
      });

      this.listenTo(view, 'remove-click', e =>
        this.collection.remove(e.view.model));

      return view;
    }

  get $addCoupon () {
      return this._$addCoupon ||
        (this._$addCoupon =
          $('.js-addCoupon'));
    },

    render () {
      this.$couponsWrap = $('.js-couponsWrap');
      this._$addCoupon = null;

      this.couponViews.forEach(coupon => coupon.remove());
      this.couponViews = [];
      const couponsFrag = document.createDocumentFragment();

      this.collection.forEach(coupon => {
        const view = this.createCouponView(coupon);
        this.couponViews.push(view);
        view.render().$el.appendTo(couponsFrag);
      });

      this.$couponsWrap.append(couponsFrag);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
