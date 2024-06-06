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
      <FormError v-if="ob.errors['productID']" :errors="ob.errors['productID']" />
      <input
        type="text"
        class="clrBr clrP clrSh2"
        name="productID"
        v-model="formData.productID"
        :placeholder="ob.polyT('editListing.variantInventory.placeholderSKU')"
        :maxlength="ob.max?.productIdLength"
      />
    </td>
    <td class="clrBr">
      <FormError v-if="ob.errors['images']" :errors="ob.errors['images']" />
      <input type="file" id="inputPhotoUpload" ref="inputPhotoUpload" @change="onChangePhotoUploadInput" accept="image/*" class="hide" multiple />
      <ul ref="photoUploadItems" class="unstyled uploadItems clrBr rowSm js-photoUploadItems">
        <li class="addElement tile js-addPhotoWrap">
          <span class="imagesIcon ion-images clrT4"></span>
          <button class="btn clrP clrBr clrT tx6" @click="$refs.inputPhotoUpload.click()">{{ ob.polyT('editListing.btnAddPhoto') }}</button>
        </li>
        <template v-for="(image, j) in formData.images" :key="image.id">
          <UploadPhoto :image="image" @closeIcon="onClickRemoveImage(j)" />
        </template>
      </ul>
    </td>
    <td class="clrBr">
      <a class="iconBtn clrBr clrP clrSh2 margLSm btnRemoveVariant" @click="onClickRemove"><i class="ion-trash-b"></i> </a>
    </td>
  </tr>
</template>

<script>
import _ from 'underscore';
import bigNumber from 'bignumber.js';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      formData: {
        name: '',
        surcharge: 0,
        productID: '',
        images: [],
      },
    };
  },
  created() {},
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
    // Sets the model based on the current data in the UI.
    setModelData() {
      const formData = this.formData;
      if (!_.isEmpty(formData.surcharge)) {
        formData.surcharge = bigNumber(formData.surcharge);
      }
      this.model.set(formData, { validate: true });
    },
  },
};
</script>
<style lang="scss" scoped>
.imagesIcon {
  font-size: 50px;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  top: 35%;
}
</style>
