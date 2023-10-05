<template>
  <a :class="`js-tab tab row contractMenuItem ${active ? 'clrT active' : ''}`">
    <span class="contractLabel">{{ ob.polyT('orderDetail.navMenu.contract') }}</span>
    <span v-if="ob.tip" class='${ob.tipClass || ""}' data-tip="${ob.tip}"><span class="${ob.tipIconClass}"></span></span>
  </a>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {},
	  },
    active: {
      type: Boolean,
      default: false,
	  },
  },

  data () {
    return {
      _state: {
        tip: '',
        tipClass: 'toolTip clrTAlert',
        tipIconClass: 'ion-alert-circled padSm',
      }
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    }
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        ...options,
        initialState: {
          tip: '',
          tipClass: 'toolTip clrTAlert',
          tipIconClass: 'ion-alert-circled padSm',
          ...options.initialState || {},
        },
      };

      this.baseInit(opts);
    },
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
