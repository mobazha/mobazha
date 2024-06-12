<template>
  <ul :class="`unstyled errorList js-errorList ${ob.class}`" :id="id">
    <li v-for="(err, k) in errors" :key="k">
      <i class="ion-alert-circled"></i>{{ err }}
    </li>
  </ul>
</template>
  
<script setup>
const props = defineProps({
  errors: Array,
  id: String,
  class: String,
})
</script>
<style lang="scss" scoped></style>