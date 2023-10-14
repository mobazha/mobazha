<template>
  <div>
    <span :class="ob.className">
      <CryptoIcon :code="ob.fromCur"/>
      <span :class="`fromCur ${ob.fromCurClass}`">{{ ob.fromCur }}</span>

      <span :class="`pairingSeparator ion-android-arrow-forward ${ob.arrowIconClass || ''}`"></span>
      
      <template v-if="ob.toCur">
        <CryptoIcon :code="ob.toCur"/>
        <span :class="`fromCur ${ob.toCurClass}`">{{ ob.toCur }}</span>
      </template>
    </span>
  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {
        className: 'cryptoTradingPairLg',
        arrowIconClass: '',
        truncateCurAfter: 8,
        fromCur: '',
        toCur: '',
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
    },
  },
  methods: {
  }
}
</script>
<style lang="scss" scoped></style>