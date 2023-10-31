<template>
  <tr>
    <template v-for="(choice, j) in ob.choices" :key="j">
      <td class="clrBr">{{ choice }}</td>
    </template>
    <td class="clrBr">
      <FormError v-if="ob.errors['surcharge']" :errors="ob.errors['surcharge']" />
      <input type="text" class="clrBr clrP clrSh2" @change="onChangeSurchange($event)" name="surcharge"
        :value="ob.number.toStandardNotation(formData.surcharge)" placeholder="0.00" data-var-type="bignumber" />
    </td>
    <td class="clrBr js-totalPrice">{{ ob.calculateTotalPrice(formData.surcharge || ob.bigNumber('0')) }}</td>
    <td class="clrBr">
      <FormError v-if="ob.errors['productID']" :errors="ob.errors['productID']" />
      <input type="text" class="clrBr clrP clrSh2" name="productID" v-model="formData.productID" :placeholder="ob.polyT('editListing.variantInventory.placeholderSKU')" :maxlength="ob.max.productIdLength" />
    </td>
    <td class="clrBr unconstrainedWidth quantityCol">
      <FormError v-if="ob.errors['quantity']" :errors="ob.errors['quantity']" />
      <div class="flexVCent gutterH">
        <input type="text" class="clrBr clrP clrSh2 " @focus="onFocusQuantity" @keyup="onKeyupQuantity" name="quantity"
          :value="formData.infiniteInventory ? infiniteQuantityChar : formData.quantity" data-var-type="bignumber" placeholder="0" />
        <input type="checkbox" :id="`${ob.cid}_inventoryItemUnlimtedCheckbox`" class="centerLabel" v-model="formData.infiniteInventory" name="infiniteInventory">
        <label class="tx5b flexNoShrink" :for="`${ob.cid}_inventoryItemUnlimtedCheckbox`">{{ ob.polyT('editListing.variantInventory.unlimitedQuantityLabel') }}</label>
      </div>
    </td>
  </tr>
</template>

<script>
import { formatCurrency } from '../../../../backbone/utils/currency';
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
      infiniteQuantityChar: '—',

      formData: {
        surcharge: 0,
        productID: '',
        quantity: '',
        infiniteInventory: true,
      }
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
        errors: {
          ...(this.model.validationError || {}),
        },
        calculateTotalPrice: this.calculateTotalPrice.bind(this),
        cid: this.model.cid,
        max: this.model.max,
      };
    },
  },
  methods: {
    initFormData() {
      const model = this.model.toJSON();
      this.formData = {
        surcharge: model.surcharge,
        productID: model.productID,
        quantity: model.quantity,
        infiniteInventory: model.infiniteInventory,
      }
    },
    loadData () {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.initFormData();
    },

    onChangeSurchange(event) {
      this.formData.surcharge = event.target.value;
    },

    onFocusQuantity (e) {
      if (e.target.value === this.infiniteQuantityChar) {
        e.target.setSelectionRange(0, e.target.value.length);
      }
    },

    onKeyupQuantity (e) {
      if (e.target.value !== this.infiniteQuantityChar) {
        this.formData.infiniteInventory = false;
        this.formData.quantity = e.target.value;
      } else {
        this.formData.infiniteInventory = true;
        this.formData.quantity = '';
      }
    },

    // Sets the model based on the current data in the UI.
    setModelData () {
      const formData = this.formData;

      if (formData.infiniteInventory) {
        delete formData.quantity;
        this.model.unset('quantity');
      }

      this.model.set(formData);
    },

    calculateTotalPrice (surcharge) {
      const listingPrice = this.options.basePrice;

      let formatted;

      try {
        formatted = formatCurrency(bigNumber(listingPrice).plus(surcharge), this.options.listingCurrency);
      } catch (e) {
        return '';
      }

      return formatted;
    },
  }
}
</script>
<style lang="scss" scoped></style>
