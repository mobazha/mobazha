<template>
  <div :class="modelContentClass" @keydown.esc="clickEsc">
    <span :class="`${closeButtonClass} jsModalClose`" @click="close" v-show="showCloseButton" :data-tip="closeButtonTip || ''">
      <i :class="innerButtonClass"></i>
    </span>
    <slot name="component"></slot>
  </div>
</template>

<script>
import _ from 'underscore';
import app from '../../../backbone/app';

export default {
  props: {
    modalInfo: {
      type: Object,
      default: {}
    }
  },
  created() {
    _.extend(this.$data, this.modalInfo);
  },
  data () {
    return {
      // #259 - we've decided not have modals close on an overlay click, so you
      // probably should never be passing in true for this.
      dismissOnOverlayClick: false,
      dismissOnEscPress: true,
      showCloseButton: true,
      closeButtonClass: 'cornerTR iconBtn clrP clrBr clrSh3 toolTipNoWrap modalCloseBtn',
      innerButtonClass: 'ion-ios-close-empty',
      closeButtonTip: app.polyglot.t('pageNav.toolTip.close'),
      modelContentClass: 'modalContent',
      removeOnClose: false,
      removeOnRoute: true,
    }
  },
  watch: {
    $route (to, from) {
      if (this.removeOnRoute && (to.path !== from.path)) {
        this.close();
      }
    }
  },
  methods: {
    close() {
      this.$emit('close');
    },
    clickEsc() {
      if (this.dismissOnEscPress) {
        this.close();
      }
    },
  }
}
</script>