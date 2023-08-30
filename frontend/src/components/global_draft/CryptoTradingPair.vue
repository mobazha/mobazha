<template>
  <div>
    <span :class="ob.className">
      {{ 
        ob.polyT('cryptoCodeIconPairing', {
          code: `<span class="fromCur ${ob.fromCurClass || ''}">${ob.fromCur}</span>`,
          icon: ob.crypto.cryptoIcon({ code: ob.fromCur }),
        })  
      }}
      <span :class="`pairingSeparator ion-android-arrow-forward ${ob.arrowIconClass || ''}`"></span>
      <div v-if="ob.toCur">
        {{ 
          ob.polyT('cryptoCodeIconPairing', {
            code: `<span class="fromCur ${ob.toCurClass || ''}">${ob.toCur}</span>`,
            icon: ob.crypto.cryptoIcon({ code: ob.toCur }),
          }) 
        }}
      </div>
    </span>
  </div>
</template>

<script setup>
const props = defineProps({
  feeLevel: String,
})
</script>
<style lang="scss" scoped>
</style>