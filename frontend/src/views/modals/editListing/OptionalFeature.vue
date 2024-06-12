<template>
  <tr>
    <td class="clrBr">
      <FormError v-if="ob.errors['name']" :errors="ob.errors['name']" />
      <input type="text" class="clrBr clrP clrSh2" v-model="formData.name" />
    </td>
    <td class="clrBr">
      <FormError v-if="ob.errors['surcharge']" :errors="ob.errors['surcharge']" />
      <input type="number" class="clrBr clrP clrSh2" v-model="formData.surcharge" placeholder="0.00" data-var-type="bignumber" />
    </td>
    <td class="clrBr">
      <FormError v-if="ob.errors['skuID']" :errors="ob.errors['skuID']" />
      <input
        type="text"
        class="clrBr clrP clrSh2"
        name="skuID"
        v-model="formData.skuID"
        :placeholder="ob.polyT('editListing.variantInventory.placeholderSKU')"
        :maxlength="ob.max?.productIdLength"
      />
    </td>
    <td class="clrBr">
      <FormError v-if="ob.errors['image']" :errors="ob.errors['image']" />
      <UploadPhoto2 :image="formData.images[0]" @imageChange="onImageChange" />
    </td>
    <td class="clrBr">
      <a class="iconBtn clrBr clrP clrSh2 margLSm btnRemoveVariant" @click="onClickRemove"><i class="ion-trash-b"></i> </a>
    </td>
  </tr>
</template>

<script>
import bigNumber from 'bignumber.js';
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
      formData: {
        name: '',
        surcharge: 0,
        skuID: '',
        images: [undefined],
      },
    };
  },
  created() {
    this.loadData();
  },
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        errors: {
          ...(this.model?.validationError || {}),
        },
        max: this.model?.max,
      };
    },
  },
  methods: {
    initFormData() {
      const model = this.model.toJSON();
      this.formData = {
        name: model.name,
        surcharge: model.surcharge,
        skuID: model.skuID,
        images: model.images,
      };
    },
    loadData() {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.initFormData();
    },

    onClickRemove() {
      this.$emit('removeClick', this.model);
    },

    // Sets the model based on the current data in the UI.
    setModelData() {
      const formData = this.formData;
      if (formData.surcharge != null) {
        formData.surcharge = bigNumber(formData.surcharge);
      }
      this.model.set(formData, { validate: true });
    },

    onImageChange(image) {
      this.formData.images[0] = image;
    }
  },
};
</script>
<style lang="scss" scoped>
.imageIcon {
  font-size: 50px;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  top: 35%;
}
</style>
