<template>
  <li :style="`background-image: url('${ob.getServerUrl(`ob/image/${image.small}`)}')`" class="tile">
    <div class="primaryIndicatorWrap">
      <div class="clrE1 clrTOnEmph phraseBox padTn">{{ ob.polyT('editListing.primaryPhoto') }}</div>
    </div>
    <a class="closeIcon tx2" @click="onCloseIcon">
      <span class="ion-ios-close-empty clrBr clrP clrT"></span>
    </a>
  </li>
</template>

<script>
export default {
  props: {
    image: {
      type: Object,
      default: {
        small: ''
      },
    },
  },
  data () {
    return {
    };
  },
  methods: {
    onCloseIcon() {
      this.$emit('closeIcon')
    }
  },
}

</script>
<style lang="scss" scoped></style>
