<template>
  <!-- // Add a 'processing' class to the button for the spinner to appear. -->

  <a :class="`btnProcessing ${className}`" :data-tip="dataTip">
    <SpinnerSVG :className="spinnerClassName || ''" />
    <span :class="`js-btnText ${textClassName}`">{{ btnText }}</span>
  </a>

</template>

<script setup>
const props = defineProps({
  className: String,
  btnText: String,
  dataTip: String,
  spinnerClassName: String,
  textClassName: String,
})
</script>
<style lang="scss" scoped>
</style>