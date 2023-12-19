<template>
  <div class="modal modalScrollPage page-container">
    <BaseModal @close="onClose">
      <template v-slot:component>
        <div class="page-main">
          <div class="page-head">
            <div class="page-head__left">
              <div class="page-head__name">
                Favorite <span v-if="tableData.length > 0">({{ cartNum }})</span>
              </div>
              <div class="clean-btn" v-if="tableData.length > 0" @click="clearCart">Clear All</div>
            </div>
            <!-- <div class="page-head__right" v-if="tableData.length > 0">
              <el-input v-model="params.keyword" placeholder="Search Orders" prefix-icon="Search" />
            </div> -->
          </div>
          <div class="page-body" v-loading="loading">
            <div>
              <p>{{ ob.polyT('shoppingCart.header.notice') }}</p>
              <p>{{ ob.polyT('shoppingCart.header.notice1') }}</p>
              <p>{{ ob.polyT('shoppingCart.header.notice2') }}</p>
            </div>
            <template v-if="tableData.length > 0">
              <div class="table-hc">
                <el-table :header-row-style="headerRowStyle" :data="[]" :height="38">
                  <el-table-column width="48"></el-table-column>
                  <el-table-column label="Title" width="200"></el-table-column>
                  <el-table-column label="Type" width="150"></el-table-column>
                  <el-table-column label="Variants" width="160"></el-table-column>
                  <el-table-column label="Price"></el-table-column>
                  <el-table-column label="Quantity" width="100"></el-table-column>
                  <el-table-column label="Total"></el-table-column>
                  <el-table-column label="Operate" width="100"></el-table-column>
                </el-table>
              </div>
              <div class="card">
                <div class="card-item" v-for="(item, index) in tableData" :key="index">
                  <el-table ref="table" :header-cell-style="headerCellStyle" class="table-hearder-one" :data="item.items"
                    @selection-change="handleSelectionChange($event, index)"
                  >
                    <el-table-column>
                      <template #header>
                        <div class="user">
                          <img class="user-avatar" :src="ob.getAvatarBgImage(item.profile?.avatarHashes, {}, true)" @click="goToStore(item.vendorID)" />
                          <div class="user-body">
                            <div class="user-name" @click="goToStore(item.vendorID)">{{ item.profile?.name }}</div>
                            <div class="user-id">{{ item.vendorID }}</div>
                          </div>
                        </div>
                      </template>
                      <template #default>
                        <el-table-column type="selection" width="48"> </el-table-column>
                        <el-table-column width="200">
                          <template v-slot="{ row }">
                            <div class="goods">
                              <div class="goods-left">
                                <img
                                  class="goods-img"
                                  :src="ob.getListingBgImage(row.listing?.item.images[0], {}, true)"
                                  @click="goToListing(item.vendorID, row.listing?.slug)"
                                />
                              </div>
                              <div class="goods-right">
                                <div class="goods-title" @click="goToListing(item.vendorID, row.listing?.slug)">{{ row.listing?.item.title }}</div>
                                <div class="goods-currency">
                                  <img
                                    class="currency-icon"
                                    :src="`../../imgs/cryptoIcons/${currency}-icon.png`"
                                    v-for="(currency, index) in row.listing?.metadata.acceptedCurrencies"
                                    :key="index"
                                  />
                                </div>
                              </div>
                            </div>
                          </template>
                        </el-table-column>
                        <el-table-column prop="type" width="150"></el-table-column>
                        <el-table-column width="160">
                          <template v-slot="{ row }">
                            <div class="sku">
                              <div class="sku-item" v-for="(val, key) in row.options" :key="key">
                                <div class="sku-name">{{ val.name }}</div>
                                <div class="sku-value">{{ val.value }}</div>
                              </div>
                            </div>
                          </template>
                        </el-table-column>
                        <el-table-column prop="price"></el-table-column>
                        <el-table-column width="100">
                          <template v-slot="{ row }">
                            <el-input class="input-number" v-model="row.quantity" />
                          </template>
                        </el-table-column>
                        <el-table-column>
                          <template v-slot="{ row }">{{ countRowPrice(row) }}</template>
                        </el-table-column>
                        <el-table-column width="100">
                          <template v-slot="{ row }">
                            <el-button @click="doDelete(row, index)" type="info" icon="Delete" circle />
                          </template>
                        </el-table-column>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div class="footer" v-if="oneStoreTotalPrice(index).quantity > 0">
                    <div class="total">
                      <div class="total-price"><span class="total-name">Total: </span>{{ oneStoreTotalPrice(index).total }}</div>
                      <div v-if="false" class="count-price">Subtotal:$183.97</div>
                      <div v-if="false" class="freight">Shipping & handling: Free</div>
                    </div>
                    <button class="btn-primary pay-btn small" @click="pay(index)">{{ ob.polyT('listingDetail.buyNow') }}</button>
                  </div>
                </div>
              </div>
            </template>
          </div>
          <Empty v-if="tableData.length === 0 && !loading" :emptyInfo="emptyInfo" />
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import { ElMessage, ElMessageBox } from 'element-plus';
import $ from 'jquery';
import app from '../../backbone/app';
import Empty from '@/components/Empty.vue';
import api from '../api';
import { getCachedProfiles } from '../../backbone/models/profile/Profile';
import { convertCurrency, formatCurrency, convertAndFormatCurrency } from '../../backbone/utils/currency';
import Purchase from './modals/purchase/Purchase.vue';
import Listing from '../../backbone/models/listing/Listing';
import OrderListings from '../../backbone/collections/OrderListings';

export default {
  components: {
    Empty,
    Purchase,
  },
  name: 'App',
  data() {
    return {
      params: { keyword: '' },
      tableData: [],
      table: {},
      selectors: {}, //选中的商品
      loading: false,

      emptyInfo: {
        type: 'shoppingCart',
        icon: new URL('@/assets/img/empty/cart.png', import.meta.url).href,
        name: 'Your cart is empty!',
        desc: "The possibilities are endless. Go ahead, find something you'll love.",
        btn: 'Shop Popular Products',
      },
    };
  },
  created() {
    this.loadData();
  },
  computed: {
    localCurrency() {
      return app.settings.get('localCurrency');
    },

    //每个商品总价
    countRowPrice() {
      return (row) =>
        row.priceAmount ? convertAndFormatCurrency(row.priceAmount * row.quantity, row.pricingCurrency?.code, this.localCurrency) : 0;
    },

    //每个商店商品总价
    oneStoreTotalPrice() {
      return (index) => {
        let list = this.selectors[index];
        if (!list) return 0;

        const total = list.reduce((cur, next) => cur + convertCurrency(next.priceAmount * next.quantity, next.pricingCurrency?.code, this.localCurrency), 0);
        return { quantity: list.length, total: formatCurrency(total, this.localCurrency) };
      };
    },

    //购物车商品总数量
    cartNum() {
      return this.tableData.reduce((cur, next) => cur + next.items.length, 0);
    },
  },
  methods: {
    onClose() {
      this.$emit('close');
    },
    loadData() {
      try {
        // this.loading = true;
        api.getShoppingCarts().then((carts) => {
          this.tableData = carts;

          let fetches = [];
          this.tableData.forEach((cart) => {
            getCachedProfiles([cart.vendorID])[0].done((profile) => {
              cart.profile = profile.toJSON();
            });

            cart.listings = [];
            cart.items?.forEach((item) => {
              item.listingExt = new Listing({ slug: item.slug }, { guid: cart.vendorID, hash: item.listingHash });
              cart.listings.push(item.listingExt);

              item.vendorID = cart.vendorID;

              const listingFetch = item.listingExt.fetch();
              fetches.push(listingFetch);
            });
          });

          $.whenAll(fetches.slice()).always(() => {
            this.tableData.forEach((cart) => {
              cart.items?.forEach((item) => {
                let listing = item.listingExt.toJSON();
                item.listing = listing;
                item.pricingCurrency = listing.metadata.pricingCurrency;
                if (listing.item.price && item.pricingCurrency) {
                  item.priceAmount = listing.item.price;
                  item.price = convertAndFormatCurrency(item.priceAmount, item.pricingCurrency.code, this.localCurrency);
                } else {
                  item.priceAmount = 0;
                  item.price = 0;
                }
                item.type = app.polyglot.t(`formats.${listing.metadata.contractType}`)
              });
            });

            this.loading = false;
          });
        });
        // this.tableData = products;
      } catch {
        this.loading = false;
      }
    },

    //删除单个商品
    doDelete(row, index) {
      ElMessageBox.confirm(app.polyglot.t('shoppingCart.deleteConfirm.body'), app.polyglot.t('shoppingCart.deleteConfirm.heading'), {
        confirmButtonText: app.polyglot.t('shoppingCart.deleteConfirm.btnDelete'),
        cancelButtonText: app.polyglot.t('shoppingCart.deleteConfirm.btnCancel'),
        type: 'warning',
        callback: (action) => {
          if (action === 'confirm') {
            //this.tableData.splice(index, 1)为展示效果，调用删除接口，再刷新
            api.removeCartItem(row.vendorID, {
              slug: row.listing?.slug,
              options: row.options,
            }).then(() => {
              ElMessage({ type: 'success', message: app.polyglot.t('shoppingCart.deleteConfirm.tip', {item: row.listing?.item.title}) });
              this.loadData();
            }).fail((jqXHR) => {
              ElMessage({ type: 'error', message: jqXHR?.responseJSON?.reason });
            })
          }
        },
      });
    },

    goToStore(peerID) {
      window.location = `#${peerID}/store`;
    },

    goToListing(vendorID, slug) {
      window.location = `#${vendorID}/store/${slug}`;
    },

    handleSelectionChange(val, index) {
      this.selectors[index] = val;
    },

    clearCart() {
      api.clearShoppingCarts({}, () => {
        this.$store.commit('cart/updateCart', {}, { module: 'cart' });
        this.loadData();
      });
    },

    //提交当前选中的商店商品
    pay(index) {
      this.$store.commit('cart/updateCart', this.tableData[0], { module: 'cart' });

      const item = this.tableData[index];
      const vendor = {
        peerID: item.vendorID,
        name: item.profile?.name,
        handle: item.profile?.handle,
        avatarHashes:  item.profile?.avatarHashes,
      };

      const itemsToPurchase = new OrderListings();
      const purchaseInfo = [];

      const rows = this.selectors[index];
      rows.forEach((row) => {
        itemsToPurchase.push(row.listingExt);

        purchaseInfo.push({quantity: row.quantity, variants: row.options});
      });

      window.vueApp.launchPurchaseModal({itemsInfo: purchaseInfo, vendor}, () => {
        return {itemsToPurchase};
      });
    },

    //修改头部样式
    headerRowStyle({ rowIndex }) {
      if (rowIndex === 0) return { background: 'transparent', color: '#000', fontSize: '16px' };
    },

    headerCellStyle({ rowIndex }) {
      if (rowIndex === 1) return { display: 'none' };
    },
  },
};
</script>

<style lang="scss" scoped>
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 31px 0 30px 0;

  &__left {
    display: flex;
    align-items: center;
  }

  &__name {
    color: #000;
    font-size: 30px;
    line-height: normal;
    margin-right: 14px;
  }

  .clean-btn {
    cursor: pointer;
    text-decoration: underline;
    color: #787878;
    font-size: 14px;

    &:hover {
      opacity: 0.8;
    }
  }
}

.card {
  &-item {
    padding: 0 20px 20px 20px;
    border: 1px solid #e0e0e0;
    background: #fff;

    &:not(:last-child) {
      margin-bottom: 20px;
    }
  }
}

.user {
  display: flex;
  align-items: center;

  &-avatar {
    cursor: pointer;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    flex-shrink: 0;
    margin-right: 10px;
  }

  &-body {
    flex: 1;
  }

  &-name {
    cursor: pointer;
    text-decoration: underline;
    font-size: 16px;
    color: #000000;

    &:hover {
      opacity: 0.8;
    }
  }

  &-id {
    word-break: break-all;
    font-size: 14px;
    color: #787878;
  }
}

.goods {
  display: flex;

  &-left {
    width: 64px;
    height: 64px;
    flex-shrink: 0;
    margin-right: 12px;
  }

  &-img {
    cursor: pointer;
    width: 100%;
    height: 100%;
  }

  &-right {
    flex: 1;
  }

  &-title {
    cursor: pointer;
    text-decoration: underline;
    color: #000;
    font-size: 14px;
    line-height: 20px;
    margin-bottom: 8px;

    &:hover {
      opacity: 0.8;
    }
  }

  .currency-icon {
    width: 16px;
    height: 16px;

    &:not(:last-child) {
      margin-right: 4px;
    }
  }
}

.sku {
  &-item {
    display: flex;
    line-height: 16px;
  }

  &-name {
    color: #000;
    font-size: 14px;
    min-width: 50px;

    &::after {
      content: ':';
    }
  }

  &-value {
    word-break: break-all;
    color: #666;
    font-size: 14px;
  }
}

.row-price {
  color: #00a054;
  font-variant-numeric: lining-nums proportional-nums;
  font-family: Helvetica;
  font-size: 14px;
  font-weight: bold;
  line-height: 32px;
  letter-spacing: 0.14px;
}

.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 0 0 10px;

  .total {
    &-name {
      color: #000;
      font-size: 20px;
      font-weight: bold;
      line-height: 24px;
    }

    &-price {
      color: #00a054;
      font-size: 20px;
      font-weight: bold;
      line-height: 24px;
      margin-bottom: 5px;
    }
  }

  .count-price,
  .freight {
    color: #000;
    font-size: 14px;
    line-height: 20px;
  }
}
::v-deep() {
  @import '../assets/scss/module/table.scss';
  @import '../assets/scss/module/input.scss';
}
</style>
