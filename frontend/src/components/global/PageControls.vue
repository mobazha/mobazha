<template>
  <div class="pageControlsWrapper overflowAuto">

    <div class="floR">
      <div class="pageControls flexVCent gutterH tx5">
        <template v-if="countsAvailable">
          <div v-html="ob.polyT('pageControls.displaying', {
            displayingCounts: ob.polyT('pageControls.displayingCounts', {
              start: ob.number.localizeNumber(ob.start),
              end: ob.number.localizeNumber(ob.end),
              total: ob.number.localizeNumber(ob.total),
            }),
          })">
          </div>
        </template>
        <div class="btnStrip">
          <button class="btn clrP clrBr pagePrev" @click="onClickPrev" :disabled="disabledPrev">
            <i class="ion-arrow-left-b"></i>
          </button>
          <button class="btn clrP clrBr pageNext" @click="onClickNext" :disabled="disabledNext">
            <i class="ion-arrow-right-b"></i>
          </button>
        </div>
      </div>
    </div>

  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {
        start: 1,
        end: 0,
        total: 0,
      },
    },
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.options,
      };
    },
    countsAvailable () {
      const ob = this.ob;

      let countsAvailable = false;

      if (typeof ob.start === 'number' &&
        typeof ob.end === 'number' &&
        typeof ob.total === 'number') {
        countsAvailable = true;
      }
      return countsAvailable;
    },
    disabledPrev () {
      const ob = this.ob; 
      let disabledPrev = true;

      if (this.countsAvailable) {
        if (ob.start > 1) {
          disabledPrev = false;
        }
      }
      return disabledPrev;
    },
    disabledNext () {
      const ob = this.ob;
      let disabledNext = true;

      if (this.countsAvailable) {
        if (ob.end < ob.total) {
          disabledNext = false;
        }
      }
      return disabledNext;
    }
  },
  methods: {
    onClickNext () {
      this.$emit('clickNext');
    },

    onClickPrev () {
      this.$emit('clickPrev');
    },

  }
}
</script>
<style lang="scss" scoped></style>
