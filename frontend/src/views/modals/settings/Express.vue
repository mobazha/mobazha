<template>
  <div class="panel">
    <div class="panel-title required">{{ ob.polyT('editListing.sectionNames.shipping') }}</div>
    <div class="panel-box">
      <div class="panel-item" v-for="(item, index) in shippingOptions" :key="index">
        <div class="panel-head">
          <div class="panel-head__name">{{ ob.polyT('editListing.shippingOptions.optionHeading', { listPosition: index + 1 }) }}</div>
          <button class="btn" @click="onClickRemoveShippingOption">{{ ob.polyT('editListing.shippingOptions.btnDeleteShippingOption') }}</button>
        </div>
        <hr class="clrBr rowMd" />
        <!-- <slot :item="{ ...item, $index: index }" /> -->
        <ShopingOptionsDetail v-if="item.options?.length" :data="{ templateId: item.templateId, options: item.options }" />
        <section>
          <button class="btn btn-text" @click="addExpressInfo(item, index)">+添加服务</button>
        </section>
      </div>
      <div class="panel-item">
        <div class="panel-head">
          <div class="panel-head__name">{{ ob.polyT('editListing.shippingOptions.optionHeading', { listPosition: shippingOptions.length + 1 }) }}</div>
        </div>
        <hr class="clrBr rowMd" />
        <div class="add">
          <button @click="doAdd" class="btn add-btn">{{ ob.polyT('editListing.shippingOptions.btnAddShippingOption') }}</button>
          <div class="helper">{{ ob.polyT('editListing.helperShipping') }}</div>
        </div>
      </div>
    </div>
  </div>
  <ShoppingOptionsModal ref="modal" @getExpressInfo="getExpressInfo" />
</template>
<script>
import ShoppingOptionsModal from './ShoppingOptionsModal.vue';
import ShopingOptionsDetail from './ShopingOptionsDetail.vue';

export default {
  components: { ShoppingOptionsModal, ShopingOptionsDetail },
  data() {
    return {
      expressInfo: {},
    };
  },
  emits: ['onClickAddShippingOption'],
  computed: {},
  props: {
    ob: Object,
    shippingOptions: {
      type: [Array],
      default: () => [],
    },
  },
  methods: {
    addExpressInfo(item) {
      this.expressInfo = item;
      this.$refs.modal.open();
    },
    doAdd() {
      this.$emit('onClickAddShippingOption');
    },
    doDelete() {
      this.$emit('click-remove', this.model);
    },
    getExpressInfo({ templateId, options }) {
      this.expressInfo.templateId = templateId;
      this.expressInfo.options = options;
    },
  },
};
</script>
<style lang="scss" scoped>
.panel {
  padding: 15px;
  &-title {
    margin: 15px 0;
  }
  &-head {
    display: flex;
    justify-content: space-between;
    padding: 5px;
    &__name {
      color: hsl(0, 0%, 14.8%);
      font-weight: bold;
      margin-bottom: 7.5px;
    }
  }

  &-item {
    padding: 10px;
    border-radius: 2px;
    border: 1px solid #dbdbdb;
    font-size: 1.5rem;
    word-wrap: break-word;
    box-shadow: 0 1px 1px rgb(0 0 0 / 30%);
    &:not(:last-child) {
      margin-bottom: 15px;
    }
    .add-btn {
      margin-bottom: 5px;
    }
  }
}
.btn {
  color: hsl(0, 0%, 14.8%);
  box-shadow: 0 1px 0 rgb(0 0 0 / 5%);
  border-color: #dbdbdb;
  background-color: hsl(0, 0%, 100%);
  margin-right: 5px;
}
.btn-text {
  height: initial;
  border: none;
  padding: 0;
  margin: 0;
  background: transparent;
  color: hsl(117, 66%, 41%);
  font-weight: 400;
  box-shadow: none;
  cursor: pointer;
  &:hover {
    text-decoration: underline;
  }
}
.helper {
  color: #777777;
  font-size: 1.1rem;
}
</style>
