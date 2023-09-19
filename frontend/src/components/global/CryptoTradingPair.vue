<template>
  <div>
    <span :class="ob.className">
      <CryptoIcon :code="ob.fromCur"/>
      <span :class="`fromCur ${ob.fromCurClass}`">{{ ob.fromCur }}</span>

      <span :class="`pairingSeparator ion-android-arrow-forward ${ob.arrowIconClass || ''}`"></span>
      
      <div v-if="ob.toCur">
        <CryptoIcon :code="ob.toCur"/>
        <span :class="`fromCur ${ob.toCurClass}`">{{ ob.toCur }}</span>
      </div>
    </span>
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
    this.loadData(this.$props.options);
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
  },
  methods: {
    loadData(options = {}) {
      this.setState(options || {});
    }
  }
}
</script>
<style lang="scss" scoped></style>