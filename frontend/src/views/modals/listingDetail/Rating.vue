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
