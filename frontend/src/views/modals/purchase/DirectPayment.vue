<template>
  <div class="moderatorsWrapper fauxModeratorsWrapper">
    <!-- // duplicate the moderator card html to make sure everything aligns -->

    <div class="moderatorCard clrBrInvis clickable" @click="clickDirectPurchase">
      <div class="moderatorCardInner">
        <div class="flexRow gutterH moderatorCardContent">
          <div class="flexNoShrink">
            <div class="btnRadio">
              <div tabindex="0" :class="`fauxRadioBtn ${active ? 'active' : ''}`"></div>
            </div>
          </div>
          <div class="moderatorCardMiddle">
            <div class="flex rowSm">
              <b>{{ ob.polyT('purchase.directPayment') }}</b>
            </div>
            <div class="clrT2">{{ ob.polyT('purchase.directPaymentDetails') }}</div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>


export default {
  props: {
    active: {
      type: Boolean,
      default: false,
	  },
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
  },

  methods: {
    clickDirectPurchase () {
      this.$emit('click');
    },
  }
}
</script>
<style lang="scss" scoped></style>
