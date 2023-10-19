<template>
  <div class="ratingStrip">
    <div class="ratingIcons">
      <!-- // Important!!! In order to simulate a previous sibling selector in
      // CSS and hover all the previos icons on hover, the icons are
      // displayed in reverse order via flex-direction. This requires index
      // calculations to be computed from the end. -->
      <template v-for="(val, key) in ob.maxRating" :key="key">
        <a :class="`ratingIcon js-ratingIcon ${ob.clickable ? 'clickable' : ''}`" :selected="ob.curRating > ob.maxRating - val"
          @click="onClickRatingIcon(val)"
          v-html="ob.parseEmojis('⭐', ob.iconClrClass)">
        </a>
      </template>
    </div>
    <span :class="`ratingNumbers ${ob.numberClrClass}`">({{ ob.curRating }}/{{ ob.maxRating }})</span>
  </div>
</template>

<script>
import _ from 'underscore';

export default {
  props: {
    options: {
      type: Object,
      default: {
        curRating: 0,
        clickable: false,
        maxRating: 5,
      },
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
        ...this.options,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this._state = {
        hoverIndex: 0,
        iconClrClass: '',
        numberClrClass: 'clrT2',
        ...options.initialState || {},
      };
    },

    onClickRatingIcon (val) {
      // Important!!! In order to simulate a previous sibling selector in
      // CSS and hover all the previos icons on hover, the icons are
      // displayed in reverse order via flex-direction. This requires index
      // calculations to be computed from the end.
      this.setState({ curRating: this.maxRating - val });
    },
  }
}
</script>
<style lang="scss" scoped></style>
