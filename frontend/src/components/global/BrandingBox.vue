<template>
  <div :class="`brandingBoxWrap ${ob.wrapClass}`">
    <div class="brandingLogo" style="background-image: url('../imgs/obVectorIconSmall2.png');"></div>
    <div class="brandingText">Mobazha</div>
  </div>
</template>

<script setup>
</script>
<style lang="scss" scoped>
.brandingBoxWrap {
  @extend .flexVCent;

  .brandingLogo {
    width: 50px;
    height: 50px;
    background-repeat: no-repeat;
    margin-right: 3px;
  }

  .brandingText {
    font-size: $tx3;
  }
}
</style>