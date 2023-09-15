<template>
  <div class="pageControlsWrapper overflowAuto">

    <div class="floR">
      <div class="pageControls flexVCent gutterH tx5">
        <div v-if="countsAvailable">
          <div> {{
            ob.polyT('pageControls.displaying', {
              displayingCounts: ob.polyT('pageControls.displayingCounts', {
                start: ob.number.localizeNumber(ob.start),
                end: ob.number.localizeNumber(ob.end),
                total: ob.number.localizeNumber(ob.total),
              }),
            })
          }}
          </div>
        </div>
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
import loadTemplate from '../../../backbone/utils/loadTemplate';


export default {
  props: {
    options: {
      type: Object,
      default: {},
	},
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
    params () {
      return {
        type: this.type,
        ...this.getState(),
      };
    },
    countsAvailable () {
      let countsAvailable = false;

      if (typeof ob.start === 'number' &&
        typeof ob.end === 'number' &&
        typeof ob.total === 'number') {
        countsAvailable = true;
      }
      return countsAvailable;
    },
    disabledPrev () {
      let disabledPrev = true;

      if (countsAvailable) {
        if (ob.start > 1) {
          disabledPrev = false;
        }
      }
      return disabledPrev;
    },
    disabledNext () {
      let disabledNext = true;

      if (countsAvailable) {
        if (ob.end < ob.total) {
          disabledNext = false;
        }
      }
      return disabledNext;
    }
  },
	methods: {
  loadData(options = {}) {
    const opts = {
      ...options,
      initialState: {
        start: 1,
        ...options.initialState,
      },
    };

    super(opts);
  },

  className() {
    return 'pageControlsWrapper overflowAuto';
  },

  events() {
    return {
                };
  },

  onClickNext() {
    this.$emit('clickNext');
  },

  onClickPrev() {
    this.$emit('clickPrev');
  },

  render() {
    loadTemplate('components/pageControls.html', (t) => {
      this.$el.html(t({
        type: this.type,
        ...this.getState(),
      }));
    });

    return this;
  }

  }
}
</script>
<style lang="scss" scoped>
</style>
