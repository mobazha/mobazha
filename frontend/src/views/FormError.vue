<template>
  <div>
    <ul :class="`unstyled errorList js-errorList ${ob.class || ''}`" :id="ob.id">
      <li v-for="(err, key) in ob.errors" :key="key"><i class="ion-alert-circled"></i>{{ err }}</li>
    </ul>
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
    };
  },
  created () {

  },
  mounted () {
  },
  computed: {
  },
  methods: {

  }
}
</script>
<style lang="scss" scoped></style>
