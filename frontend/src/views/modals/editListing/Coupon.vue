<template>
  <div class="coupon flexRow gutterH">
    <div class="col4 simpleFlexCol">
      <FormError v-if="ob.errors['title']" :errors="ob.errors['title']" />
      <input type="text" ref="title" class="clrBr clrP clrSh2 marginTopAuto" v-model="formData.title"
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
          <input type="text" class="clrBr clrP clrSh2" v-model="formData.discountAmount" placeholder="0.00" >
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
      discountType: '',

      formData: {},
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
    createFormData(model) {
      return {
        title: model.title,
        discountCode: model.discountCode,
        discountAmount: model.discountAmount,
      }
    },
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.baseInit(options);
    },

    setFocus() {
      this.$refs.title.focus();
    },

    onClickRemove () {
      this.$emit('remove-click', this.model);
    },

    getFormDataEx () {
      const fields = this.$el.querySelectorAll('select[name], input[name], textarea[name]');
      const formData = this.getFormData(fields);

      if (this.discountType === 'FIXED') {
        const bigNumDiscount = bigNumber(formData.discountAmount);
        formData.priceDiscount = bigNumDiscount.isNaN() ?
          formData.discountAmount : bigNumDiscount;
      } else {
        // discountAmount
        const percentDiscount = Number(formData.discountAmount);

        formData.percentDiscount = formData.discountAmount && !isNaN(percentDiscount) ?
          percentDiscount : formData.discountAmount;
      }

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
  }
}
</script>
<style lang="scss" scoped></style>
