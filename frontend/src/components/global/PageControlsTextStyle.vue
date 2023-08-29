<template>
  <div class="pageControlsWrapper overflowAuto">

    <div class="floR">
      <div class="pageControls flexVCent gutterH tx5">
        <div class="btnStrip">
          <button class="btn clrP clrBr pagePrev" @click="onClickPrev" :disabled="ob.currentPage === 1">
            <span class="txUnb">{{ '< ' + ob.polyT('pageControlsTextStyle.previous') }}</span>
          </button>
          <div v-if="typeof ob.currentPage === 'number'">
            <div class="btn clrP clrBr unclickable">
              <span class="txUnb">{{ ob.number.localizeNumber(ob.currentPage) }}</span>
            </div>
          </div>
          <button class="btn clrP clrBr pageNext" @click="onClickNext" :disabled="!ob.morePages">
            <span class="txUnb">{{ ob.polyT('pageControlsTextStyle.next') }} ></span>
          </button>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup>
import loadTemplate from '../../../backbone/utils/loadTemplate';


const props = defineProps({
  phase: String,
})

loadData(props);

render();

function loadData (options = {}) {
  const opts = {
    ...options,
    initialState: {
      start: 1,
      ...options.initialState,
    },
  };

  super(opts);
}

function onClickNext () {
  this.trigger('clickNext');
}

function onClickPrev () {
  this.trigger('clickPrev');
}

function render () {
  loadTemplate('components/pageControlsTextStyle.html', (t) => {
    this.$el.html(t({
      ...this.getState(),
    }));
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
