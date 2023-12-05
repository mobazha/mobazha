<template>
  <div class="walletSeed gutterH">
    <template v-if="!ob.seed">
      <ProcessingButton :className="`btn clrP clrBr clrSh2 js-showSeed ${ob.isFetching ? 'processing' : ''}`"
        @click="onClickShowSeed" :btnText="ob.polyT('settings.advancedTab.server.walletSeed.btnShowSeed')" />
    </template>

    <template v-else>
      <p>{{ ob.polyT('settings.advancedTab.server.walletSeed.introLine') }}</p>
      <p class="pad border clrBr clrP">{{ ob.seed }}</p>
      <p>{{ ob.polyT('settings.advancedTab.server.walletSeed.directionLine') }}</p>
      <p class="txB row">{{ ob.polyT('settings.advancedTab.server.walletSeed.warningHeading') }}</p>
      <ul class="seedWarnings">
        <li>{{ ob.polyT('settings.advancedTab.server.walletSeed.warning1') }}</li>
        <li>{{ ob.polyT('settings.advancedTab.server.walletSeed.warning2') }}</li>
        <li>{{ ob.polyT('settings.advancedTab.server.walletSeed.warning3') }}</li>
        <li>{{ ob.polyT('settings.advancedTab.server.walletSeed.warning4') }}</li>
      </ul>
    </template>

  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {
        seed: '',
        isFetching: false,
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
        seed: '',
        isFetching: false,
        ...this.options,
      };
    }
  },
  methods: {
    onClickShowSeed () {
      this.$emit('clickShowSeed');
    },
  }
}
</script>
<style lang="scss" scoped></style>
