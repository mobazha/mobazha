<template>
  <div :class="`brandingBoxWrap ${ob.wrapClass}`">
    <img class="brandingLogo" src="~@/../imgs/obVectorIconSmall2.png" />
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
