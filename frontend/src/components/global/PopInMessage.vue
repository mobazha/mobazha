<template>
  <div>

    <a v-if="ob.dismissable" class="closeIcon tx2 js-dismiss">
      <span class="ion-ios-close-empty clrBr clrP clrT clrBrT"></span>
    </a>

    <p v-if="ob.messageText" class="txUnl">{{ ob.messageText }}</p>
    <div v-else>
      {{ ob.messageHTML }}
    </div>

  </div>
</template>

<script setup>
const props = defineProps({
  phase: String,
})

</script>
<style lang="scss" scoped>
</style>