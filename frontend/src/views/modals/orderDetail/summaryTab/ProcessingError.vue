<template>
  <div class="rowLg clrTErr">
    <div v-if="ob.isBuyer || ob.isModerator">
      <div v-if="!ob.isModerated">
        <p v-if="!ob.errors.length"><span class="ion-alert-circled padSm"></span>{{
          !ob.isOrderCancelable ?
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsg') :
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsgCancelable')
        }}</p>

        <div v-else>
          <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrBuyer') }}</p>
          <ul class="row">
            <li v-for="(err, j) in ob.errors" :key="j">{{ err }}</li>
          </ul>
          <div v-if="ob.isOrderCancelable">
            <p>{{ ob.polyT('orderDetail.summaryTab.processingError.youMayCancel') }}</p>
          </div>
        </div>
      </div>
      <div v-else>

        <p v-if="!ob.errors.length"><span class="ion-alert-circled padSm"></span>{{
          !ob.isDisputable ?
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsg') :
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsgDisputable')
        }}</p>

        <div v-else>
          <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrBuyer') }}</p>
          <ul class="row"> <li v-for="(err, j) in ob.errors" :key="j">{{ err }}</li></ul>
          <div v-if="ob.isDisputable && !ob.isModerator">
            <p>{{ ob.polyT('orderDetail.summaryTab.processingError.youMayDispute') }}</p>
          </div>
        </div>

      </div>
    </div>
    <div v-else>
      <!-- it's the vendor -->
      <div v-if="!ob.errors.length">
        <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrVendorNoMsg') }}</p>
      </div>

      <div v-else>
        <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrVendor') }}</p>
        <ul>
          <li v-for="(err, j) in ob.errors" :key="j">{{ err }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script>

export default {
  mixins: [],
  props: {
    cart: Object,
  },
  data () {
    return {
      isBuyer: false,
      isModerated: false,
      isOrderCancelable: false,
      isDisputable: false,
      errors: [],
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide the order id.');
      }

      this.orderID = options.orderID;
    },
  }
}
</script>
<style lang="scss" scoped></style>
