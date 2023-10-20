<template>
  <div class="ratingStrip">
    <template v-if="ob.fetched" >
      <div v-html="ob.formatRating(ob.averageRating, ob.ratingCount)" />
    </template>
  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      averageRating: 0,
      ratingCount: 0,
      fetched: false,
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
        averageRating: this.averageRating,
        ratingCount: this.ratingCount,
        fetched: this.fetched,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.averageRating = options.averageRating || 0;
      this.ratingCount = options.ratingCount || 0;
      this.fetched = options.fetched || false;
    },
  }
}
</script>
<style lang="scss" scoped></style>
