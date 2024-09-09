<template>
  <div :class="`cryptoIcon crypto-icon ${className}`">
    <i class="crypto-icon__large" :style="style"></i>
    <i v-if="style2" class="crypto-icon__small" :style="style2"></i>
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
      defaultIcon: 'default-coin-icon.png',
    };
  },
  created() {},
  mounted() {},
  computed: {
    style() {
      const baseIconPath = '/imgs/cryptoIcons/';

      const iconUrl = this.code ? `url(${baseIconPath}${this.code}-icon.png),` : '';
      const defaultIcon = this.defaultIcon ? `url(${baseIconPath}${this.defaultIcon})` : '';

      return `background-image: ${iconUrl}${defaultIcon}`;
    },
    style2() {
      const baseIconPath = '/imgs/cryptoIcons/';

      const coinData = getCurrencyByCode(this.code);

      if (!coinData || !coinData.mainChain) {
        return '';
      }

      const iconUrl = `url(${baseIconPath}${coinData.mainChain}-icon.png),`;
      const defaultIcon = this.defaultIcon ? `url(${baseIconPath}${this.defaultIcon})` : '';

      return `background-image: ${iconUrl}${defaultIcon}`;
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
</style>
