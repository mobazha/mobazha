<template>
  <div class="coupon flexRow gutterH">
    <div class="col4 simpleFlexCol">
      <FormError v-if="ob.errors['title']" :errors="ob.errors['title']" />
      <input type="text" ref="title" class="clrBr clrP clrSh2 marginTopAuto" name="title" v-model="formData.title"
        :placeholder="ob.polyT('editListing.coupons.titlePlaceholder')" :maxlength="ob.max.titleLength">
    </div>
    <div class="col4 simpleFlexCol">
      <FormError v-if="ob.errors['discountCode']" :errors="ob.errors['discountCode']" />
      <input type="text" class="clrBr clrP clrSh2 marginTopAuto" v-model="formData.discountCode"
        :placeholder="ob.polyT('editListing.coupons.discountCodePlaceholder')">
    </div>
    <div class="col4 simpleFlexCol">
      <FormError v-if="ob.errors['percentDiscount']" :errors="ob.errors['percentDiscount']" />
      <FormError v-if="ob.errors['priceDiscount']" :errors="ob.errors['priceDiscount']" />
      <div class="flexRow marginTopAuto">
        <div class="inputSelect marginTopAuto">
          <input type="number" class="clrBr clrP clrSh2" @input="onInputDiscountAmount" :value="discountAmount" placeholder="0.00" >
            <!-- // disables the search box -->
          <Select2 class="clrBr clrP nestInputRight" v-model="discountType" :options="{ minimumResultsForSearch: Infinity }">
            <option value="PERCENTAGE" :selected="!ob.priceDiscount">{{
              ob.polyT('editListing.coupons.discountTypePercent') }}</option>
            <option value="FIXED" :selected="!!ob.priceDiscount">{{ ob.polyT('editListing.coupons.discountTypeFixed') }}
            </option>
          </Select2>
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
import bigNumber from 'bignumber.js';
import { toStandardNotation } from '../../../../backbone/utils/number';

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
      formData: {
        title: '',
        discountCode: '',
        percentDiscount: undefined,
        priceDiscount: undefined,
      },

      discountType: '',
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
        ...this.model.toJSON(),
        max: this.model.max,
        errors: this.model.validationError || {},
      };
    },
    discountAmount () {
      if (this.formData.priceDiscount) {
        return toStandardNotation(this.formData.priceDiscount);
      } else if (typeof this.formData.percentDiscount !== 'undefined') {
        return toStandardNotation(this.formData.percentDiscount);
      }
    }
  },
  methods: {

    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.baseInit(options);

      const model = this.model.toJSON();
      this.formData = {
        title: model.title,
        discountCode: model.discountCode,
        percentDiscount: model.percentDiscount,
        priceDiscount: model.priceDiscount,
      }
      this.discountType = !this.formData.priceDiscount ? 'PERCENTAGE' : 'FIXED';
    },

    setFocus() {
      this.$refs.title.focus();
    },

    onClickRemove () {
      this.$emit('remove-click', this.model);
    },

    onInputDiscountAmount(event) {
      const discountAmount = event.target.value;

      if (this.discountType === 'FIXED') {
        const bigNumDiscount = bigNumber(discountAmount);
        this.formData.priceDiscount = bigNumDiscount.isNaN() ? discountAmount : bigNumDiscount;

        this.formData.percentDiscount = undefined;
      } else {
        // discountAmount
        const percentDiscount = Number(discountAmount);
        this.formData.percentDiscount = discountAmount && !isNaN(percentDiscount) ? percentDiscount : discountAmount;

        this.formData.priceDiscount = undefined;
      }
    },

    // Sets the model based on the current data in the UI.
    setModelData () {
      if (this.formData.priceDiscount !== undefined) {
        this.model.unset('percentDiscount');
      } else {
        this.model.unset('priceDiscount');
      }

      this.model.set(this.formData);
    },
  }
}
</script>
<style lang="scss" scoped></style>
