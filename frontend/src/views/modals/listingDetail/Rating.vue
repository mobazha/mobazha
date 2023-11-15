<template>
  <div class="ratingStrip">
    <div v-if="ob.fetched" v-html="ob.formatRating(ob.averageRating, ob.ratingCount)" />
  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {
        averageRating: 0,
        ratingCount: 0,
        fetched: false,
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
    }
  },
  methods: {
  }
}
</script>
<style lang="scss" scoped></style>
