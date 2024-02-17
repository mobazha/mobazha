<template>
  <div class="sendReceiveNav clrP">
    <div class="flexBtnWrapper flexBtnTop flexRow">
      <button :class="['btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm', { active: activeTab === 'send' }]" @click="changeTab('send', 'Wallet_SendShow')">
        {{ ob.polyT('wallet.sendBtn') }}
      </button>
      <button :class="['btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm', { active: activeTab === 'receive' }]" @click="changeTab('receive', 'Wallet_ReceiveShow')">
        {{ ob.polyT('wallet.receiveBtn') }}
      </button>
      <button v-if="activeCoin !== 'MATICMBZ'" :class="['btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm', { active: activeTab === 'external' }]" @click="changeTab('external', 'Wallet_External')">
        {{ ob.polyT('wallet.externalBtn') }}
      </button>
    </div>
  </div>
</template>

<script>
import { recordEvent } from '../../../../backbone/utils/metrics';

export default {
  props: {
    activeTab: {
      type: String,
      default: 'send',
    },
    activeCoin: {
      type: String,
      default: 'MATICUSDT'
    }
  },
  data() {
    return {};
  },
  created() {},
  mounted() {},
  computed: {},
  methods: {
    changeTab(val, eventName) {
      this.$emit('changeTab', val);
      recordEvent(eventName);
    },
  },
};
</script>
<style lang="scss" scoped></style>
