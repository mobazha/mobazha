<template>
  <div class="modal modalScrollPage">
    <BaseModal>
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
import { ipc } from '../../utils/ipcRenderer.js';
import 'velocity-animate';

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
    this.loadData(this.options);
  },
  mounted () {
  },
  unmounted() {
    ipc.removeListener('server-log', this.onServerConnectLog);
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
      const opts = {
        debugLog: app.serverLog,
        ...options,
      };

      this.baseInit(opts);
    },

    onCopyClick () {
      ipc.send('controller.system.writeToClipboard', this.debugLog);
      $('.js-copiedConfirm')
        .velocity('stop')
        .velocity('fadeIn')
        .velocity('fadeOut', { delay: 1000 });
    },
  }
}
</script>
<style lang="scss" scoped></style>
