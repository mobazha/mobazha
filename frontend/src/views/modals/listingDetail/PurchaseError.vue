<template>
  <div>
    <span class="clrTErr">{{ ob.polyT('listingDetail.unableToPurchase.unpurchaseable') }}</span>
    <span class="arrowBoxTipWrap">
      <span class="clrTErr">
        <span class="ion-help-circled clrTErr"></span>
      </span>
      <div class="arrowBox arrowBoxCenteredTop clrP clrBr js-tipContent" v-html="tip">
      </div>
    </span>
  </div>
</template>

<script>

export default {
  props: {
    tip: {
      type: String,
      default: '',
    },
  },
}
</script>
<style lang="scss" scoped></style>
