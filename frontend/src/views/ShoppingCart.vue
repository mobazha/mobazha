<template>
  <div class="modal modalScrollPage">
    <BaseModal>
      <template v-slot:component>
        <div class="page-main">
          <div class="page-head">
            <div class="page-head__left">
              <div class="page-head__name">
                Shopping Cart<span v-if="tableData.length > 0">({{ cartNum }})</span>
              </div>
              <div class="clean-btn" v-if="tableData.length > 0">Clear Cart</div>
            </div>
            <div class="page-head__right" v-if="tableData.length > 0">
              <el-input v-model="params.keyword" placeholder="Search Orders" :prefix-icon="Search" />
            </div>
          </div>
          <div class="page-body" v-loading="loading">
            <template v-if="tableData.length > 0">
              <div class="table-hc">
                <el-table :header-row-style="headerRowStyle" :data="[]" :height="38">
                  <el-table-column width="48"></el-table-column>
                  <el-table-column label="Title" width="350"></el-table-column>
                  <el-table-column label="Type" width="160"></el-table-column>
                  <el-table-column label="Price"></el-table-column>
                  <el-table-column label="Quantity" width="100"></el-table-column>
                  <el-table-column label="Total"></el-table-column>
                  <el-table-column label="Operate" width="100"></el-table-column>
                </el-table>
              </div>
              <div class="card">
                <div class="card-item" v-for="(item, index) in tableData" :key="index">
                  <el-table :header-cell-style="headerCellStyle" ref="table" class="table-hearder-one" :data="item.items"
                    @selection-change="handleSelectionChange($event, index)">
                    <el-table-column>
                      <template #header>
                        <div class="user">
                          <img class="user-avatar" :src="getAvatarBgImage(item.profile?.avatarHashes, {}, true)"
                            @click="goToStore(item.vendorID)" />
                          <div class="user-body">
                            <div class="user-name" @click="goToStore(item.vendorID)">{{ item.profile?.name }}</div>
                            <div class="user-id">{{ item.vendorID }}</div>
                          </div>
                        </div>
                      </template>
                      <template #default>
                        <el-table-column type="selection" width="48"> </el-table-column>
                        <el-table-column width="350">
                          <template v-slot="{ row }">
                            <div class="goods">
                              <div class="goods-left">
                                <img class="goods-img" :src="getListingBgImage(row.listing?.item.images[0], {}, true)"
                                  @click="goToListing(item.vendorID, row.listing?.slug)" />
                              </div>
                              <div class="goods-right">
                                <div class="goods-title" @click="goToListing(item.vendorID, row.listing?.slug)">{{
                                  row.listing?.item.title }}</div>
                                <div class="goods-currency">
                                  <img class="currency-icon" :src="`../../imgs/cryptoIcons/${currency}-icon.png`"
                                    v-for="(currency, index) in row.listing?.metadata.acceptedCurrencies" :key="index" />
                                </div>
                              </div>
                            </div>
                          </template>
                        </el-table-column>
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
                            <el-button @click="doDelete(row, index)" type="info" :icon="Delete" circle />
                          </template>
                        </el-table-column>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div class="footer" v-if="oneStoreTotalPrice(index).quantity > 0">
                    <div class="total">
                      <div class="total-price"><span class="total-name">Total:</span>${{ oneStoreTotalPrice(index).total }}
                      </div>
                      <div class="count-price">Subtotal:$183.97</div>
                      <div class="freight">Shipping & handling: Free</div>
                    </div>
                    <button class="btn-primary pay-btn" @click="pay(index)">Pay</button>
                  </div>
                </div>
              </div>
            </template>
          </div>
          <empty v-if="tableData.length === 0 && !loading" :emptyInfo="emptyInfo" />
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script setup>
import { Delete } from '@element-plus/icons-vue';
import { Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import $ from 'jquery';
import { toRaw } from 'vue';
import app from '../../backbone/app';
import Empty from '../components/Empty.vue';
import { products } from '../components/products.js';
import api from '../api';
import { getCachedProfiles } from '../../backbone/models/profile/Profile';
import { convertAndFormatCurrency, curDefToDecimal } from '../../backbone/utils/currency';
import { getAvatarBgImage, getListingBgImage } from '../../backbone/utils/responsive';

import { useStore } from 'vuex';
import { useRouter } from 'vue-router';

const store = useStore();
const router = useRouter();

const props = defineProps({
  options: Object,
})

const params = reactive({ keyword: '' });
let tableData = [];
const table = reactive({});
const selectors = ref({}); //选中的商品
const loading = ref(false);

const emptyInfo = ref({
  type: 'shoppingCart',
  icon: new URL('@/assets/img/empty/cart.png', import.meta.url).href,
  name: 'Your cart is empty!',
  desc: "The possibilities are endless. Go ahead, find something you'll love.",
  btn: 'Shop Popular Products',
});

loadData();

function loadData () {
  try {
    loading.value = true;
    api.getShoppingCarts().then((carts) => {
      tableData = carts;
      tableData.forEach((cart) => {
        getCachedProfiles([cart.vendorID])[0].done((profile) => {
          cart.profile = profile.toJSON();
        });
        cart.items?.forEach((item) => {
          $.get(window['app']?.getServerUrl(`ob/listing/${item.listingHash}`)).then((listing) => {
            item.listing = listing.listing;
            item.cid = listing.cid;
            item.pricingCurrency = listing.listing.metadata.pricingCurrency;
            item.priceAmount = curDefToDecimal({
              amount: listing.listing.item.price,
              currency: item.pricingCurrency,
            });
            item.price = convertAndFormatCurrency(item.priceAmount, item.pricingCurrency.code, window['app']?.settings.get('localCurrency'));
          });
        });
      });
      loading.value = false;
    });
    // tableData = products;
  } catch {
    loading.value = false;
  }
}

//删除单个商品
function doDelete (row, index) {
  ElMessageBox.confirm('确定删除该商品吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action) => {
      if (action === 'confirm') {
        //tableData.splice(index, 1)为展示效果，调用删除接口，再刷新
        tableData.splice(index, 1);
        ElMessage({ type: 'success', message: '已删除' });
        loadData();
      }
    },
  });
}

//每个商品总价
const countRowPrice = computed(() => {
  return (row) => row.priceAmount ? convertAndFormatCurrency(row.priceAmount * row.quantity, row.pricingCurrency?.code, window['app']?.settings.get('localCurrency')) : 0;
});
//每个商店商品总价
const oneStoreTotalPrice = computed(() => (index) => {
  let list = selectors.value[index];
  if (!list) return 0;
  return { quantity: list.length, total: list.reduce((cur, next) => cur + next.priceAmount * next.quantity, 0) };
});
//购物车商品总数量
const cartNum = computed(() => {
  return tableData.reduce((cur, next) => cur + next.items.length, 0);
});

function goToStore (peerID) {
  window.location = `#${peerID}/store`;
}

function goToListing (vendorID, slug) {
  window.location = `#${vendorID}/store/${slug}`;
}

function handleSelectionChange (val, index) {
  selectors.value[index] = val;
}

//提交当前选中的商店商品
function pay (index) {
  store.commit('cart/updateCart', toRaw(tableData[0]), { module: 'cart' });

  app.router.loadVueModal('Purchase');
}

//修改头部样式
function headerRowStyle ({ rowIndex }) {
  if (rowIndex === 0) return { background: 'transparent', color: '#000', fontSize: '16px' };
}
function headerCellStyle ({ rowIndex }) {
  if (rowIndex === 1) return { display: 'none' };
}
</script>

<style lang="scss" scoped>
@import '../assets/scss/main.scss';

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
</style>
