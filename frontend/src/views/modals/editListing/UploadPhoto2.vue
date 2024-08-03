<template>
  <div>
    <div v-show="!!imageUpload">
      {{ ob.polyT('editListing.uploading') }} <a class="" @click="onClickCancelPhotoUploads">{{ ob.polyT('editListing.btnCancelUpload') }}</a>
    </div>
    <input type="file" ref="inputPhotoUpload" @change="onChangePhotoUploadInput" accept="image/*" class="hide" />

    <div class="unstyled clrBr">
      <div v-if="!image" class="addElement customTile">
        <span class="imageIcon ion-images clrT4"></span>
        <button class="btn clrP clrBr clrT tx6" @click="$refs.inputPhotoUpload.click()">{{ ob.polyT('editListing.btnAddPhoto') }}</button>
      </div>
      <div v-if="!!image" class="customTile">
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
import { myAjax } from '../../../api/api';
import { truncateImageFilename } from '../../../../backbone/utils/index';

export default {
  props: {
    image: {
      type: Object,
      default: undefined,
    },
  },
  data () {
    return {
      imageUpload: undefined,
    };
  },
  methods: {
    onImageChange(image) {
      this.$emit('imageChange', image);
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
      this.imageUpload = myAjax({
        url: app.getServerUrl('ob/productimages'),
        type: 'POST',
        data: JSON.stringify([image]),
        dataType: 'json',
        contentType: 'application/json',
      })
        .done((uploadedImages) => {
          if (this.isRemoved()) return;

          const resultImage = uploadedImages[0];
          this.onImageChange({
              filename: resultImage.filename,
              original: resultImage.original,
              large: resultImage.large,
              medium: resultImage.medium,
              small: resultImage.small,
              tiny: resultImage.tiny,
            });
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

    onCloseIcon() {
      this.onImageChange(undefined);
    }
  },
}

</script>
<style lang="scss" scoped>
@import '../../../../styles/variables';
@import '../../../../styles/mixins';
.customTile {
  // width: 60px;
  // height: 60px;
  border: 1px solid;
  border-color: inherit;
  border-radius: $corner;
  float: left;
  margin-right: $pad;
  margin-bottom: $pad;
  position: relative;

  .closeIcon {
    width: 24px;
    height: 24px;
    position: absolute;
    top: 0;
    left: 0;
  }

  .btn {
    width: 35px;
    height: 28px;
    position: absolute;
    bottom: 10px;
    padding: 0;
    line-height: 1;
    @include center(true, false);
  }
}
</style>
