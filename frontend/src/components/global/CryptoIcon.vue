
<template>
  <i class="cryptoIcon ${className}" :style="style"></i>
</template>

<script>
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
  data () {
    return {
      defaultIcon: 'default-coin-icon.png',
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    style () {
      const baseIconPath = '../../../imgs/cryptoIcons/';

      const iconUrl = this.code ? `url(${baseIconPath}${this.code}-icon.png),` : '';
      const defaultIcon = this.defaultIcon ? `url(${baseIconPath}${this.defaultIcon})` : '';

      return `background-image: ${iconUrl}${defaultIcon}`;
    },
  },
  methods: {
  },
}
</script>
<style lang="scss" scoped></style>