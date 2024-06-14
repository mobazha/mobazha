<template>
  <tr>
    <td class="clrBr">
      <UploadPhoto2 :image="formData.images[0]" @imageChange="onImageChange" />
    </td>
    <template v-for="(choice, j) in ob.choices" :key="j">
      <td class="clrBr">{{ choice }}</td>
    </template>
    <td class="clrBr">
      <FormError v-if="ob.errors['surcharge']" :errors="ob.errors['surcharge']" />
      <input type="number" class="clrBr clrP clrSh2" v-model="formData.surcharge" placeholder="0.00" data-var-type="bignumber" />
    </td>
    <td class="clrBr js-totalPrice">{{ ob.calculateTotalPrice(formData.surcharge || ob.bigNumber('0')) }}</td>
    <td class="clrBr">
      <FormError v-if="ob.errors['productID']" :errors="ob.errors['productID']" />
      <input
        type="text"
        class="clrBr clrP clrSh2"
        name="productID"
        v-model="formData.productID"
        :placeholder="ob.polyT('editListing.variantInventory.placeholderSKU')"
        :maxlength="ob.max.productIdLength"
      />
    </td>
    <td class="clrBr unconstrainedWidth quantityCol">
      <FormError v-if="ob.errors['quantity']" :errors="ob.errors['quantity']" />
      <div class="flexVCent gutterH">
        <input type="number" class="clrBr clrP clrSh2" v-model="formData.quantity" :placeholder="quantityPlaceholder" data-var-type="bignumber" />
        <input
          type="checkbox"
          :id="`${ob.cid}_inventoryItemUnlimtedCheckbox`"
          class="centerLabel"
          v-model="formData.infiniteInventory"
          @change="changeInfinite"
        />
        <label class="tx5b flexNoShrink" :for="`${ob.cid}_inventoryItemUnlimtedCheckbox`">{{
          ob.polyT('editListing.variantInventory.unlimitedQuantityLabel')
        }}</label>
      </div>
    </td>
    <td class="clrBr">
      <a class="iconBtn clrBr clrP clrSh2 margLSm btnRemoveVariant" @click="onClickRemove"><i class="ion-trash-b"></i> </a>
    </td>
  </tr>
</template>

<script>
import bigNumber from 'bignumber.js';
import { formatCurrency } from '../../../../backbone/utils/currency';
import UploadPhoto2 from './UploadPhoto2.vue';

export default {
  components: {
    UploadPhoto2,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      infiniteQuantityChar: '99999999',

      formData: {
        surcharge: 0,
        productID: '',
        quantity: '',
        infiniteInventory: true,
        images: [undefined],
      },
    };
  },
  created() {
    this.loadData(this.options);
  },
  mounted() {},
  computed: {
    ob() {
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
    quantityPlaceholder() {
      if (this.formData.infiniteInventory) {
        return this.infiniteQuantityChar;
      } else {
        return 0;
      }
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
        images: model.images,
      };

      if (this.formData.infiniteInventory) {
        this.formData.quantity = this.infiniteQuantityChar;
      }
    },
    loadData() {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.initFormData();
    },

    changeInfinite() {
      if (this.formData.infiniteInventory) {
        this.formData.quantity = '';
      } else {
        this.formData.quantity = 0;
      }
    },

    // Sets the model based on the current data in the UI.
    setModelData() {
      const formData = this.formData;

      if (formData.surcharge != null) {
        formData.surcharge = bigNumber(formData.surcharge);
      }

      if (formData.infiniteInventory) {
        delete formData.quantity;
        this.model.unset('quantity');
      } else if (formData.quantity != null) {
        formData.quantity = bigNumber(formData.quantity);
      }

      this.model.set(formData, { validate: true });
    },

    calculateTotalPrice(surcharge) {
      const listingPrice = this.options.basePrice;

      let formatted;

      try {
        formatted = formatCurrency(bigNumber(listingPrice).plus(surcharge), this.options.listingCurrency);
      } catch (e) {
        return '';
      }

      return formatted;
    },

    onImageChange(image) {
      this.formData.images[0] = image;
    },

    onClickRemove() {
      this.$emit('removeClick', this.model);
    },
  },
};
</script>
