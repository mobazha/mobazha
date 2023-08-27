<template>

<div class="floR">
  <div class="pageControls flexVCent gutterH tx5">
    <div class="btnStrip">
      <button class="btn clrP clrBr pagePrev js-pagePrev" :disabled="ob.currentPage === 1">
        <span class="txUnb">{{ '< ' + ob.polyT('pageControlsTextStyle.previous') }}</span>
      </button>

      <div v-if="typeof ob.currentPage === 'number'" class="btn clrP clrBr unclickable">
        <span class="txUnb">{{ ob.number.localizeNumber(ob.currentPage) }}</span>
      </div>

      <button class="btn clrP clrBr pageNext js-pageNext" :disabled="!ob.morePages">
        <span class="txUnb">{{ ob.polyT('pageControlsTextStyle.next') }} ></span>
      </button>
    </div>
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