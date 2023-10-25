<template>
  <div class="coupon flexRow gutterH">
    <div class="col4 simpleFlexCol">
      <FormError v-if="ob.errors['title']" :errors="ob.errors['title']" />
      <input type="text" class="clrBr clrP clrSh2 marginTopAuto" name="title" :value="ob.title"
        :placeholder="ob.polyT('editListing.coupons.titlePlaceholder')" :maxlength="ob.max.titleLength">
    </div>
    <div class="col4 simpleFlexCol">
      <FormError v-if="ob.errors['discountCode']" :errors="ob.errors['discountCode']" />
      <input type="text" class="clrBr clrP clrSh2 marginTopAuto" name="discountCode" :value="ob.discountCode"
        :placeholder="ob.polyT('editListing.coupons.discountCodePlaceholder')">
    </div>
    <div class="col4 simpleFlexCol">
      <FormError v-if="ob.errors['percentDiscount']" :errors="ob.errors['percentDiscount']" />
      <FormError v-if="ob.errors['priceDiscount']" :errors="ob.errors['priceDiscount']" />
      <div class="flexRow marginTopAuto">
        <div class="inputSelect marginTopAuto">
          <input type="text" class="clrBr clrP clrSh2" name="discountAmount" placeholder="0.00"
            v-model="inputDiscountAmount">
          <select name="discountType" class="clrBr clrP nestInputRight">
            <option value="PERCENTAGE" :selected="!ob.priceDiscount">{{
              ob.polyT('editListing.coupons.discountTypePercent') }}</option>
            <option value="FIXED" :selected="!!ob.priceDiscount">{{ ob.polyT('editListing.coupons.discountTypeFixed') }}
            </option>
          </select>
        </div>

        <a class="iconBtn clrBr clrP clrSh2 margLSm toolTipNoWrap  btnRemoveCoupon" @click="onClickRemove"
          :data-tip="ob.polyT('editListing.coupons.toolTip.delete')">
          <i class="ion-trash-b"></i>
        </a>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import bigNumber from 'bignumber.js';


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
      inputDiscountAmount: this.discountAmount,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();

    $('select[name=discountType]')
      .select2({
        // disables the search box
        minimumResultsForSearch: Infinity,
      });
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        max: this.model.max,
        errors: this.model.validationError || {},
      };
    },
    discountAmount () {
      const ob = this.ob;
      let discountAmount;

      if (ob.priceDiscount) {
        discountAmount = ob.number.toStandardNotation(ob.priceDiscount);
      } else if (typeof ob.percentDiscount !== 'undefined') {
        discountAmount = ob.number.toStandardNotation(ob.percentDiscount);
      }
      return discountAmount;
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.baseInit(options);
    },

    onClickRemove () {
      this.$emit('remove-click', { view: this });
    },

    getFormDataEx (fields = this.$formFields) {
      const formData = this.getFormData(fields);

      if (formData.discountType === 'FIXED') {
        const bigNumDiscount = bigNumber(formData.discountAmount);
        formData.priceDiscount = bigNumDiscount.isNaN() ?
          formData.discountAmount : bigNumDiscount;
      } else {
        // discountAmount
        const percentDiscount = Number(formData.discountAmount);

        formData.percentDiscount = formData.discountAmount && !isNaN(percentDiscount) ?
          percentDiscount : formData.discountAmount;
      }

      delete formData.discountType;
      delete formData.discountAmount;

      return formData;
    },

    // Sets the model based on the current data in the UI.
    setModelData () {
      const formData = this.getFormDataEx();

      if (formData.priceDiscount !== undefined) {
        this.model.unset('percentDiscount');
      } else {
        this.model.unset('priceDiscount');
      }

      this.model.set(formData);
    },

    get $inputDiscountAmount () {
      return this._$inputDiscountAmount ||
        (this._$inputDiscountAmount =
          $('input[name=discountAmount]'));
    },

    get $formFields () {
      return this._$formFields ||
        (this._$formFields =
          $('select[name], input[name], textarea[name]'));
    },

    render () {
      this._$formFields = null;
      this._$inputDiscountAmount = null;

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
