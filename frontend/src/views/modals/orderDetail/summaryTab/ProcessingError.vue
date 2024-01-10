<template>
  <div class="rowLg clrTErr">
    <template v-if="ob.isBuyer || ob.isModerator">
      <template v-if="!ob.isModerated">
        <p v-if="!ob.errors.length"><span class="ion-alert-circled padSm"></span>{{
          !ob.isOrderCancelable ?
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsg') :
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsgCancelable')
        }}</p>

        <template v-else>
          <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrBuyer') }}</p>
          <ul class="row">
            <li v-for="(err, j) in ob.errors" :key="j">{{ err }}</li>
          </ul>
          <template v-if="ob.isOrderCancelable">
            <p>{{ ob.polyT('orderDetail.summaryTab.processingError.youMayCancel') }}</p>
          </template>
        </template>
      </template>
      <template v-else>

        <p v-if="!ob.errors.length"><span class="ion-alert-circled padSm"></span>{{
          !ob.isDisputable ?
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsg') :
          ob.polyT('orderDetail.summaryTab.processingError.procErrBuyerNoMsgDisputable')
        }}</p>

        <template v-else>
          <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrBuyer') }}</p>
          <ul class="row"> <li v-for="(err, j) in ob.errors" :key="j">{{ err }}</li></ul>
          <template v-if="ob.isDisputable && !ob.isModerator">
            <p>{{ ob.polyT('orderDetail.summaryTab.processingError.youMayDispute') }}</p>
          </template>
        </template>

      </template>
    </template>
    <template v-else>
      <!-- it's the vendor -->
      <template v-if="!ob.errors.length">
        <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrVendorNoMsg') }}</p>
      </template>

      <template v-else>
        <p><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.summaryTab.processingError.procErrVendor') }}</p>
        <ul>
          <li v-for="(err, j) in ob.errors" :key="j">{{ err }}</li>
        </ul>
      </template>
    </template>
  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {
        isBuyer: false,
        isModerated: false,
        isOrderCancelable: false,
        isDisputable: false,
        errors: [],
      },
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.options,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide the order id.');
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
