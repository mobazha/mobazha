<template>
  <div class="modal modalScrollPage">
    <BaseModal @close="close">
      <template v-slot:component>
        <div class="topControls flex">
          <a class="btn clrP clrBr " @click="onCopyClick">{{ ob.polyT('debugLog.copyToClipboard') }}</a>
          <div class="js-copiedConfirm pad tx6 clrT2 hide">{{ ob.polyT('copiedToClipboard') }}</div>
        </div>

        <div class="contentBox padMd clrP clrBr clrSh3">
          <div class="js-debugLog preWrap tx6">{{ ob.debugLog }}</div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../backbone/app.js';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      debugLog: app.serverLog,

      maxDebugLines: 1000,
    };
  },
  created () {
    this.initEventChain();
    
    this.loadData(this.options);
  },
  mounted () {
  },
  unmounted() {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        debugLog: this.debugLog,
      };
    }
  },
  methods: {
    loadData (options = {}) {

    },

    onCopyClick () {
    },
  }
}
</script>
<style lang="scss" scoped></style>
