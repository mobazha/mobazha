<template>
  <div>
    <div class="txSm rowTn flex" v-if="ob.codeResult && ob.codeResult.type && ob.codeResult.type !== 'valid'">
      <span class="clrTErr">
        {{ 
          ob.codeResult.code
            ? ob.polyT(`purchase.codeErrors.${ob.codeResult.type}`, { code: ob.codeResult.code })
            :ob.polyT('purchase.codeErrors.blank')
        }}
      </span>
    </div>

    <div v-for="(code, i) in ob.couponCodes" :key="i" class="txSm rowTn flexVCent gutterH">
      <span class="clrTEm">{{ ob.polyT('purchase.code', { code }) }}</span>
      <button class="btnTxtOnly js-remove" :data-code="code">
        {{ ob.polyT('purchase.removeCode') }}
      </button>
    </div>

  </div>
</template>

<script setup>
</script>
<style lang="scss" scoped>
</style>