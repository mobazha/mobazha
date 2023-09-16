<template>
  <div class="sendReceiveNav clrP">
    <div class="flexBtnWrapper flexBtnTop flexRow">
      <button :class="`btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm ${ob.sendModeOn ? 'active' : ''}`" @click="onClickSend">
        {{ ob.polyT('wallet.sendBtn') }}
      </button>
      <button :class="`btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm ${!ob.sendModeOn ? 'active' : ''}`" @click="onClickReceive">
        {{ ob.polyT('wallet.receiveBtn') }}
      </button>
    </div>
  </div>
</template>

<script>
import { recordEvent } from '../../../../backbone/utils/metrics';


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

    this.loadData(this.$props.options);
  },
  mounted () {
    this.render();
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
    loadData (options = {}) {
      const opts = {
        initialState: {
          sendModeOn: true,
          ...options.initialState,
        },
      };

      this.setState(opts.initialState || {});
    },

    onClickSend () {
      this.$emit('click-send');
      recordEvent('Wallet_SendShow');
    },

    onClickReceive () {
      this.$emit('click-receive');
      recordEvent('Wallet_ReceiveShow');
    },
  }
}
</script>
<style lang="scss" scoped></style>
