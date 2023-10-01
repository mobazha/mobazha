<template>
  <div class="ratingStrip">
    <div class="ratingIcons">
      <!-- // Important!!! In order to simulate a previous sibling selector in
      // CSS and hover all the previos icons on hover, the icons are
      // displayed in reverse order via flex-direction. This requires index
      // calculations to be computed from the end. -->
      <template v-for="(i, key) in ob.maxRating" :key="key">
        <a :class="`ratingIcon ${ob.clickable ? 'clickable' : ''}`" :selected="ob.curRating > ob.maxRating - i"
          @click="onClickRatingIcon"
          v-html="ob.parseEmojis('⭐', ob.iconClrClass)">
        </a>
      </template>
    </div>
    <span :class="`ratingNumbers ${ob.numberClrClass}`">({{ ob.curRating }}/{{ ob.maxRating }})</span>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';

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

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
    rating () {
      return this._state.curRating;
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this._state = {
        curRating: 0,
        maxRating: 5,
        hoverIndex: 0,
        iconClrClass: '',
        numberClrClass: 'clrT2',
        clickable: false,
        ...options.initialState || {},
      };
    },

    onClickRatingIcon (e) {
      // Important!!! In order to simulate a previous sibling selector in
      // CSS and hover all the previos icons on hover, the icons are
      // displayed in reverse order via flex-direction. This requires index
      // calculations to be computed from the end.
      const totalIcons = this.getState().maxRating;
      this.setState({ curRating: totalIcons - $(e.target).closest('.js-ratingIcon').index() });
    },
  }
}
</script>
<style lang="scss" scoped></style>
