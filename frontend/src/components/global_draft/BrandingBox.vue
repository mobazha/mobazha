<template>
  <div :class="`brandingBoxWrap ${ob.wrapClass}`">
    <div class="brandingLogo" style="background-image: url('../imgs/obVectorIconSmall2.png')"></div>
    <div class="brandingText">Mobazha</div>
  </div>
</template>

<script>
export default {
  props: {
    wrapClass: {
      type: String,
      default: '',
    },
  },
};
</script>
<style lang="scss" scoped></style>
