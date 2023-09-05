<template>
  <a class="js-tab tab row contractMenuItem">
    <span class="contractLabel">{{ ob.polyT('orderDetail.navMenu.contract') }}</span>
    <span v-if="ob.tip" class='${ob.tipClass || ""}' data-tip="${ob.tip}"><span class="${ob.tipIconClass}"></span></span>
  </a>
</template>

<script>

export default {
  mixins: [],

  data () {
    return {
      tip: '',
      tipClass: 'toolTip clrTAlert',
      tipIconClass: 'ion-alert-circled padSm',
    };
  },
  mounted () {
  },
  computed: {
  },
  methods: {

    attributes () {
      return { 'data-tab': 'contract' };
    },

    events () {
      return {
        'click .js-openDispute': 'onClickOpenDispute',
      };
    },
  }
}
</script>
<style lang="scss" scoped></style>
