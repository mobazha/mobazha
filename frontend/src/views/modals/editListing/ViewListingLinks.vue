<template>
  <div>
    <div class="gutterH viewListingLinks">
      <a class="txU js-viewListing clrTEmph1" @click="onClickViewListing">{{ ob.polyT('editListing.viewListingInAppLink') }}</a>
      <a class="txU js-viewListingOnWeb clrTEmph1 toolTip toolTipChatOpen txCtr" @click="onClickViewListingOnWeb"
        :data-tip="ob.polyT('editListing.viewOnWebDisclaimerTooltip')">
        {{ ob.polyT('editListing.viewListingOnWebLink') }} <i class="ion-android-open padSm clrTEmph1"></i>
      </a>
    </div>
  </div>
</template>

<script>
export default {
  props: {
    image: {
      type: Object,
      default: {
      },
    },
  },
  data () {
    return {
    };
  },
  methods: {
    onClickViewListing() {
      this.$emit('viewListing')
    },
    onClickViewListingOnWeb() {
      this.$emit('viewListingOnWeb')
    }
  },
}
</script>
<style lang="scss" scoped></style>
