<template>
  <!-- // Add a 'processing' class to the button for the spinner to appear. -->

  <a :class="`btnProcessing ${className}`" v-bind="attrs">
    <SpinnerSVG :className="spinnerClassName || ''" />
    <span :class="`js-btnText ${textClassName}`" v-html="btnText"></span>
  </a>

</template>

<script setup>
const props = defineProps({
  className: String,
  btnText: String,
  attrs: {},
  spinnerClassName: String,
  textClassName: String,
})
</script>
<style lang="scss" scoped>
</style>