<template>
  <div :class="`cryptoIcon crypto-icon ${className}`">
    <i class="crypto-icon__large"><img class="bkgImg" :src="`~@/../imgs/cryptoIcons/${coin1Icon}`" /></i>
    <i v-if="coin2Icon" class="crypto-icon__small"><img class="bkgImg" :src="`~@/../imgs/cryptoIcons/${coin2Icon}`" /></i>
  </div>
</template>

<script>
import { getCurrencyByCode } from '../../../backbone/data/walletCurrencies';

export default {
  props: {
    className: {
      type: String,
      default: '',
    },
    code: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      defaultIcon: 'default-coin',
    };
  },
  created() {},
  mounted() {},
  computed: {
    coin1Icon() {
      return `${this.code ? this.code : this.defaultIcon}-icon.png`
    },
    coin2Icon() {
      const coinData = getCurrencyByCode(this.code);

      if (!coinData || !coinData.mainChain) {
        return '';
      }

      return `${coinData.mainChain}-icon.png`;
    },
  },
  methods: {},
};
</script>
<style lang="scss" scoped>
.crypto-icon {
  position: relative;
  font-size: initial;
  &__large {
    width: 100%;
    height: 100%;
    background-size: contain;
    display: inline-block;
    background-repeat: no-repeat;
    background-position: center;
  }
  &__small {
    position: absolute;
    right: -12%;
    bottom: -12%;
    width: 50%;
    height: 50%;
    background-size: contain;
    display: inline-block;
    background-repeat: no-repeat;
    background-position: center;
  }
}
.bkgImg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
