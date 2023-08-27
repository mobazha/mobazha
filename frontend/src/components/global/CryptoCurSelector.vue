<template>
  <div>

    <li class="clrBr curRow" v-for="(cur, i) in ob.processedCurs" :key="i">
      <span :class="`curControlWrapper js-curControl gutterHSm ${cur.disabled ? 'disabled' : ''}`" :data-code="cur.code">
        <input :type="ob.controlType"
          :id="`curSel${cur.code}${ob.cid}`"
          class="centerLabel"
          :name="ob.controlType === 'radio' ? 'currencies':''"
          :checked="cur.active && !cur.disabled">

        <label :for="`curSel${cur.code}${ob.cid}`">
          {{ ob.crypto.cryptoIcon({ code: cur.code }) }}
          <span class="curName noOverflow">{{ cur.displayName }}</span>
        </label>
      </span>

      <span v-if="cur.disabled && ob.disabledMsg" class="disabledMsg noOverflow clrTErr tx5b">{{ ob.disabledMsg }}</span>
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