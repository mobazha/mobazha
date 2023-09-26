<template>
  <div class="sendReceiveNav clrP">
    <div class="flexBtnWrapper flexBtnTop flexRow">
      <button :class="`btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm ${sendModeOn ? 'active' : ''}`" @click="onClickSend">
        {{ ob.polyT('wallet.sendBtn') }}
      </button>
      <button :class="`btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm ${sendModeOn ? '' : 'active'}`" @click="onClickReceive">
        {{ ob.polyT('wallet.receiveBtn') }}
      </button>
    </div>
  </div>
</template>

<script>
import { recordEvent } from '../../../../backbone/utils/metrics';

export default {
  props: {
    sendModeOn: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {};
  },
  created() {},
  mounted() {},
  computed: {},
  methods: {
    onClickSend() {
      this.$emit('clickSend');
      recordEvent('Wallet_SendShow');
    },

    onClickReceive() {
      this.$emit('clickReceive');
      recordEvent('Wallet_ReceiveShow');
    },
  },
};
</script>
<style lang="scss" scoped></style>
