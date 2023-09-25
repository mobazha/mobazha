<template>
  <div>

    <button
      :class="`clrP clrBr clrSh2 providerBtn ${ob.showSelectDefault ? 'showSelectDefault' : ''} ${ob.name ? 'toolTipNoWrap' : ''}`"
      :data-tip="ob.name">
      <div class="thumb providerInner" :style="`background-image: ${bkg}`"></div>
    </button>


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
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    bkg () {
      const ob = this.ob;

      const logo = ob.logo ? `url(${ob.logo}),` : '';
      const local = ob.localLogo ? `url(${ob.localLogo}),` : '';
      return `${logo}${local}url(../imgs/defaultProvider.png)`;
    }
  },
  methods: {

  }
}
</script>
<style lang="scss" scoped></style>
