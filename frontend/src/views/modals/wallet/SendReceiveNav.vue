<template>
  <div class="sendReceiveNav clrP">
    <div class="flexBtnWrapper flexBtnTop flexRow">
      <button :class="['btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm', { active: tabActive === 1 }]" @click="changeTab(1, 'Wallet_SendShow')">
        {{ ob.polyT('wallet.sendBtn') }}
      </button>
      <button :class="['btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm', { active: tabActive === 2 }]" @click="changeTab(2, 'Wallet_ReceiveShow')">
        {{ ob.polyT('wallet.receiveBtn') }}
      </button>
      <button :class="['btnFlx col6 flexExpand underlineOnly clrP clrBr gutterHSm', { active: tabActive === 3 }]" @click="changeTab(3, 'Wallet_External')">
        External
      </button>
    </div>
  </div>
</template>

<script>
import { recordEvent } from '../../../../backbone/utils/metrics';

export default {
  props: {
    tabActive: {
      type: Number,
      default: 1,
    },
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
