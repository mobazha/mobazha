<template>
  <div>

    <li v-for="(cur, i) in ob.processedCurs" :key="i" class="flexVCent gutterHSm clrBr">
      <span class="ion-ios-checkmark-empty clrTEm tx2"></span>
      {{ ob.crypto.cryptoIcon({ code: cur.code }) }}
      <div class="acceptedCurName">{{ cur.displayName }}</div>
    </li>

  </div>
</template>

<script setup>
const props = defineProps({
  phase: String,
})

</script>
<style lang="scss" scoped>
</style>