<template>
  <div class="ratingStrip">
    <div class="ratingIcons">
      <!-- // Important!!! In order to simulate a previous sibling selector in
      // CSS and hover all the previos icons on hover, the icons are
      // displayed in reverse order via flex-direction. This requires index
      // calculations to be computed from the end. -->
      <template v-for="(val, key) in ob.maxRating" :key="key">
        <a :class="`ratingIcon js-ratingIcon ${ob.clickable ? 'clickable' : ''} ${rating > ob.maxRating - val ? 'selected' : ''}`"
          @click="onClickRatingIcon(val)"
          v-html="ob.parseEmojis('⭐', ob.iconClrClass)">
        </a>
      </template>
    </div>
    <span :class="`ratingNumbers ${ob.numberClrClass}`">({{ rating }}/{{ ob.maxRating }})</span>
  </div>
</template>

<script>
import _ from 'underscore';

export default {
  props: {
    options: {
      type: Object,
      default: {
        clickable: false,
        maxRating: 5,
      },
    },
    rating: {
      type: Number,
      default: 0,
    },
  },
  emits: ['update:rating'],
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

        clickable: false,
        maxRating: 5,
        ...this.options,
      };
    },
  },
  methods: {
    loadData (options = {}) {
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
      this.$emit('update:rating', this.ob.maxRating - val + 1);
    },
  }
}
</script>
<style lang="scss" scoped></style>
