<template>
  <div class="pageControlsWrapper overflowAuto">
    <div class="floR">
      <div class="pageControls flexVCent gutterH tx5">
        <div class="btnStrip">
          <button class="btn clrP clrBr pagePrev" @click="onClickPrev" :disabled="ob.currentPage === 1">
            <span class="txUnb">{{ '< ' + ob.polyT('pageControlsTextStyle.previous') }}</span>
          </button>
          <template v-if="typeof ob.currentPage === 'number'">
            <div class="btn clrP clrBr unclickable">
              <span class="txUnb">{{ ob.number.localizeNumber(ob.currentPage) }}</span>
            </div>
          </template>
          <button class="btn clrP clrBr pageNext" @click="onClickNext" :disabled="!ob.morePages">
            <span class="txUnb">{{ ob.polyT('pageControlsTextStyle.next') }} ></span>
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
        currentPage: 1,
        morePages: false,
      },
    },
  },
  data() {
    return {};
  },
  created() {
    this.initEventChain();
  },
  mounted() {
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this.options,
      };
    },
  },
  methods: {
    onClickNext() {
      this.$emit('clickNext');
    },
    onClickPrev() {
      this.$emit('clickPrev');
    },
  },
};
</script>
<style lang="scss" scoped></style>
