<template>

  <div class="floR">
    <div class="pageControls flexVCent gutterH tx5">

      <div v-if="countsAvailable">{{
        ob.polyT('pageControls.displaying', {
          displayingCounts: ob.polyT('pageControls.displayingCounts', {
            start: ob.number.localizeNumber(ob.start),
            end: ob.number.localizeNumber(ob.end),
            total: ob.number.localizeNumber(ob.total),
          }),
        })
    }}
      </div>

      <div class="btnStrip">
        <button class="btn clrP clrBr pagePrev js-pagePrev" :disabled="disabledPrev">
          <i class="ion-arrow-left-b"></i>
        </button>
        <button class="btn clrP clrBr pageNext js-pageNext" :disabled="disabledNext">
          <i class="ion-arrow-right-b"></i>
        </button>
      </div>
    </div>
  </div>

</template>

<script setup>
const props = defineProps({
  phase: String,
})

var countsAvailable = false;

if (typeof ob.start === 'number' &&
  typeof ob.end === 'number' &&
  typeof ob.total === 'number') {
  countsAvailable = true;
}

var disabledPrev = true;
var disabledNext = true;

if (countsAvailable) {
  if (ob.start > 1) {
    disabledPrev = false;
  }

  if (ob.end < ob.total) {
    disabledNext = false;
  }
}

</script>
<style lang="scss" scoped>
</style>