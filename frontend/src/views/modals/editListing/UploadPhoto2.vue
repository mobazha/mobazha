<template>
  <div>
    <div v-show="!!imageUpload">
      {{ ob.polyT('editListing.uploading') }} <a class="" @click="onClickCancelPhotoUploads">{{ ob.polyT('editListing.btnCancelUpload') }}</a>
    </div>
    <input type="file" ref="inputPhotoUpload" @change="onChangePhotoUploadInput" accept="image/*" class="hide" />

    <div class="unstyled clrBr rowSm">
      <div v-if="!image" class="addElement tile js-addPhotoWrap">
        <span class="imageIcon ion-images clrT4"></span>
        <button class="btn clrP clrBr clrT tx6" @click="$refs.inputPhotoUpload.click()">{{ ob.polyT('editListing.btnAddPhoto') }}</button>
      </div>
      <div v-if="!!image" class="tile">
        <el-image
          :src="ob.getServerUrl(`ob/image/${image.small}`)"
          fit="cover"
          :preview-src-list="[ob.getServerUrl(`ob/image/${image.small}`)]"
        />
        <a class="closeIcon tx2" @click="onCloseIcon">
          <span class="ion-ios-close-empty clrBr clrP clrT"></span>
        </a>
      </div>
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import { truncateImageFilename } from '../../../../backbone/utils/index';

export default {
  props: {
  },
  data () {
    return {
      image: undefined,
      imageUpload: undefined,
    };
  },
  methods: {
    onImageChange() {
      this.$emit('imageChange', this.image);
    },

    onChangePhotoUploadInput() {
      var imageFile = this.$refs.inputPhotoUpload.files[0];

      this.$refs.inputPhotoUpload.value = '';

      const fileReader = new FileReader();
      fileReader.readAsDataURL(imageFile);

      fileReader.onload = () => {
        const image = {
          filename: truncateImageFilename(imageFile.name),
          image: fileReader.result.replace(/^data:image\/(png|jpeg|webp);base64,/, ''),
        };
    
        this.uploadImage(image);
      };

      fileReader.onerror = (error) => {
        openSimpleMessage( 'fail to read uploaded image', fileReader.error);
      };
    },

    uploadImage(image) {
      this.imageUpload = $.ajax({
        url: app.getServerUrl('ob/productimages'),
        type: 'POST',
        data: JSON.stringify([image]),
        dataType: 'json',
        contentType: 'application/json',
      })
        .done((uploadedImages) => {
          if (this.isRemoved()) return;

          this.image = uploadedImages.map((image) => ({
              filename: image.filename,
              original: image.original,
              large: image.large,
              medium: image.medium,
              small: image.small,
              tiny: image.tiny,
            }))[0];
          
          this.onImageChange();
        })
        .fail((jqXhr) => {
          openSimpleMessage(
            app.polyglot.t('editListing.errors.uploadImageErrorTitle', { smart_count: imagesToUpload.length }),
            (jqXhr.responseJSON && jqXhr.responseJSON.reason) || ''
          );
        }).always(() => (this.imageUpload = undefined));
    },

    onClickCancelPhotoUploads() {
      if (this.imageUpload) {
        this.imageUpload.abort();
        this.imageUpload = undefined;
      }
    },

    onClickRemoveImage() {
      this.image = undefined
      
      this.onImageChange();
    },

    onCloseIcon() {
      this.$emit('closeIcon')
    }
  },
}

</script>
<style lang="scss" scoped></style>
