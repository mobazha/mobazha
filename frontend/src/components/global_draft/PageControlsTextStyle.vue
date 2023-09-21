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

<script>
import loadTemplate from '../../../backbone/utils/loadTemplate';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {};
  },
  created() {
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted() {
    this.render();
  },
  computed: {
    params() {
      return {
        ...this.getState(),
      };
    },
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

      this.setState(opts.initialState || {});
    },

    events() {
      return {};
    },

    onClickNext() {
      this.$emit('clickNext');
    },

    onClickPrev() {
      this.$emit('clickPrev');
    },

    render() {
      loadTemplate('components/pageControlsTextStyle.html', (t) => {
        this.$el.html(
          t({
            ...this.getState(),
          })
        );
      });

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
