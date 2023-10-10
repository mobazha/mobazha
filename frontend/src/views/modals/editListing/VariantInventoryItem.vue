<template>
  <tr>
    <template v-for="(choice, j) in ob.choices" :key="j">
      <td class="clrBr">{{ choice }}</td>
    </template>
    <td class="clrBr">
      <FormError v-if="ob.errors['surcharge']" :errors="ob.errors['surcharge']" />
      <input type="text" class="clrBr clrP clrSh2 " @keyup="onKeyupSurcharge" name="surcharge"
        :value="ob.number.toStandardNotation(ob.surcharge)" placeholder="0.00" data-var-type="bignumber" />
    </td>
    <td class="clrBr js-totalPrice">{{ ob.calculateTotalPrice(ob.surcharge || ob.bigNumber('0')) }}</td>
    <td class="clrBr">
      <FormError v-if="ob.errors['productID']" :errors="ob.errors['productID']" />
      <input type="text" class="clrBr clrP clrSh2" name="productID" :value="ob.productID"
        :placeholder="ob.polyT('editListing.variantInventory.placeholderSKU')" :maxlength="ob.max.productIdLength" />
    </td>
    <td class="clrBr unconstrainedWidth quantityCol">
      <FormError v-if="ob.errors['quantity']" :errors="ob.errors['quantity']" />
      <div class="flexVCent gutterH">
        <input type="text" class="clrBr clrP clrSh2 " @focus="onFocusQuantity" @keyup="onKeyupQuantity" name="quantity"
          :value="ob.infiniteInventory ? ob.infiniteQuantityChar : ob.quantity" data-var-type="bignumber"
          placeholder="0" />
        <input type="checkbox" :id="`${ob.cid}_inventoryItemUnlimtedCheckbox`" class="centerLabel "
          @change="onChangeInfiniteCheckbox" name="infiniteInventory" :checked="ob.infiniteInventory">
        <label class="tx5b flexNoShrink" :for="`${ob.cid}_inventoryItemUnlimtedCheckbox`">{{
          ob.polyT('editListing.variantInventory.unlimitedQuantityLabel') }}</label>
      </div>
    </td>
  </tr>
</template>

<script>
import $ from 'jquery';
import { formatCurrency } from '../../../../backbone/utils/currency';
import loadTemplate from '../../../../backbone/utils/loadTemplate';

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
        ...this._model,
        errors: {
          ...(this.model.validationError || {}),
        },
        getCurrency: this.options.getCurrency,
        getPrice: this.options.getPrice,
        calculateTotalPrice: this.calculateTotalPrice.bind(this),
        cid: this.cid,
        infiniteQuantityChar: this.infiniteQuantityChar,
        max: this.model.max,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      if (typeof options.getPrice !== 'function') {
        throw new Error('Please provide a getPrice function that returns the product price.');
      }

      if (typeof options.getCurrency !== 'function') {
        throw new Error('Please provide a function for me to obtain the current currency.');
      }

      this.baseInit(options);
    },

    get infiniteQuantityChar () {
      return '—';
    },

    onKeyupSurcharge (e) {
      this.$totalPrice.text(this.calculateTotalPrice(e.target.value));
    },

    onFocusQuantity (e) {
      if (e.target.value === this.infiniteQuantityChar) {
        e.target.setSelectionRange(0, e.target.value.length);
      }
    },

    onKeyupQuantity (e) {
      if (e.target.value !== this.infiniteQuantityChar) {
        this.$infiniteInventoryCheckbox.prop('checked', false);
      } else {
        this.$infiniteInventoryCheckbox.prop('checked', true);
      }
    },

    onChangeInfiniteCheckbox (e) {
      if ($(e.target).is(':checked')) {
        this.$quantity.val(this.infiniteQuantityChar);
      } else {
        this.$quantity.val('');
      }
    },

    getFormDataEx (fields = this.$formFields) {
      const formData = this.getFormData(fields);
      return formData;
    },

  // Sets the model based on the current data in the UI.
  setModelData () {
      const formData = this.getFormDataEx();

      if (formData.infiniteInventory) {
        delete formData.quantity;
        this.model.unset('quantity');
      }

      this.model.set(formData);
    },

    calculateTotalPrice (surcharge) {
      const listingPrice = this.options.getPrice();

      let formatted;

      try {
        formatted = formatCurrency(
          listingPrice.plus(surcharge), this.options.getCurrency()
        );
      } catch (e) {
        return '';
      }

      return formatted;
    },

    get $formFields () {
      return this._$formFields ||
        (this._$formFields =
          $('select[name], input[name], textarea[name]'));
    },

    get $totalPrice () {
      return this._$totalPrice ||
        (this._$totalPrice =
          $('.js-totalPrice'));
    },

    get $infiniteInventoryCheckbox () {
      return this._$infiniteInventoryCheckbox ||
        (this._$infiniteInventoryCheckbox =
          $('.js-infiniteInventoryCheckbox'));
    },

    get $quantity () {
      return this._$quantity ||
        (this._$quantity =
          $('.js-quantity'));
    },

    render () {
      this._$formFields = null;
      this._$totalPrice = null;
      this._$infiniteInventoryCheckbox = null;
      this._$quantity = null;

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
