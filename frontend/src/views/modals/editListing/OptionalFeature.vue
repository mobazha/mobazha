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
      <input type="file" id="inputPhotoUpload" ref="inputPhotoUpload" @change="onChangePhotoUploadInput" accept="image/*" class="hide" multiple />
      <ul ref="photoUploadItems" class="unstyled uploadItems clrBr rowSm js-photoUploadItems">
        <li class="addElement tile js-addPhotoWrap">
          <span class="imageIcon ion-images clrT4"></span>
          <button class="btn clrP clrBr clrT tx6" @click="$refs.inputPhotoUpload.click()">{{ ob.polyT('editListing.btnAddPhoto') }}</button>
        </li>
        <UploadPhoto :image="formData.image" @closeIcon="onClickRemoveImage(j)" />
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
    bb: Function,
  },
  data() {
    return {
      formData: {
        name: '',
        surcharge: 0,
        skuID: '',
        image: undefined,
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
    onClickRemove() {
      this.$emit('removeClick', this.model);
    },

    // Sets the model based on the current data in the UI.
    setModelData() {
      const formData = this.formData;
      if (!_.isEmpty(formData.surcharge)) {
        formData.surcharge = bigNumber(formData.surcharge);
      }
      this.model.set(formData, { validate: true });
    },

    onChangePhotoUploadInput(images) {
      let photoFiles = Array.prototype.slice.call(this.$refs.inputPhotoUpload.files, 0);

      // prune out any non-image files
      photoFiles = photoFiles.filter((file) => file.type.startsWith('image'));

      this.$refs.inputPhotoUpload.value = '';

      let imagesToUpload = images;

      if (!images) {
        throw new Error('Please provide a list of images to upload.');
      }

      if (typeof images === 'string') {
        imagesToUpload = [images];
      }

      const upload = $.ajax({
        url: app.getServerUrl('ob/productimages'),
        type: 'POST',
        data: JSON.stringify(imagesToUpload),
        dataType: 'json',
        contentType: 'application/json',
      })
        .done((uploadedImages) => {
          if (this.isRemoved()) return;

          this.images.add(
            uploadedImages.map((image) => ({
              filename: image.filename,
              original: image.original,
              large: image.large,
              medium: image.medium,
              small: image.small,
              tiny: image.tiny,
            }))
          );
        })
        .fail((jqXhr) => {
          openSimpleMessage(
            app.polyglot.t('editListing.errors.uploadImageErrorTitle', { smart_count: imagesToUpload.length }),
            (jqXhr.responseJSON && jqXhr.responseJSON.reason) || ''
          );
        })
    },
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
