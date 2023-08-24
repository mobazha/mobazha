<template>
  <div class="page-container">
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
              <el-table-column label="Title" width="400"></el-table-column>
              <el-table-column label="Type" width="160"></el-table-column>
              <el-table-column label="Price"></el-table-column>
              <el-table-column label="Quantity"></el-table-column>
              <el-table-column label="Total"></el-table-column>
            </el-table>
          </div>
          <div class="card">
            <div class="card-item" v-for="(item, index) in tableData" :key="index">
              <el-table ref="table" class="table-hearder-one" :data="item.items" @selection-change="handleSelectionChange($event, index)">
                <el-table-column type="selection" width="48" rowspan="2"> </el-table-column>
                <el-table-column width="400">
                  <template #header>
                    <div class="user">
                      <img class="user-avatar" :src="item.avatar" />
                      <span class="user-name" @click="goToStore(item.vendorID)" >{{ item.name }}</span>
                      <span class="user-id">{{ item.vendorID }}</span>
                    </div>
                  </template>
                  <template v-slot="{ row }">
                    <div class="goods">
                      <div class="goods-left">
                        <img class="goods-img" :src="row.image" />
                      </div>
                      <div class="goods-right">
                        <div class="goods-title" @click="goToListing(item.vendorID, row.slug)">{{ row.title }}</div>
                        <div class="goods-currency" v-for="(currency, index) in row.acceptedCurrencies" :key="index">
                          <img class="currency-icon" :src="`../../imgs/cryptoIcons/${currency}-icon.png`" />
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
                <el-table-column>
                  <template v-slot="{ row }">
                    <el-input class="input-number" v-model="row.quantity" />
                  </template>
                </el-table-column>
                <el-table-column>
                  <template v-slot="{ row }">
                    {{ countRowPrice(row) }}
                  </template>
                </el-table-column>
              </el-table>
              <div class="footer" v-if="oneStoreTotalPrice(index).quantity > 0">
                <div class="total">
                  <div class="total-price"><span class="total-name">Total:</span>${{ oneStoreTotalPrice(index).total }}</div>
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
  </div>
</template>

<script setup>
import { Search } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import $ from 'jquery';
import Empty from './Empty.vue';
import { products } from './products.js';
import api from '../api';
import { getCachedProfiles } from '../../backbone/models/profile/Profile';

const params = reactive({ keyword: '' });
const tableData = ref([]);
const table = reactive({});
const selectors = ref({}); //选中的商品
const loading = ref(true);

const emptyInfo = ref({
  type: 'shoppingCart',
  icon: new URL('@/assets/img/empty/cart.png', import.meta.url).href,
  name: 'Your cart is empty!',
  desc: "The possibilities are endless. Go ahead, find something you'll love.",
  btn: 'Shop Popular Products',
});

setTimeout(() => {
  api.getShoppingCarts().then(carts => {
    tableData.value = carts;
    tableData.value.forEach(cart => {
      getCachedProfiles([cart.vendorID])[0].done(profile => {
        cart.name = profile.get('name');
        cart.avatar = window['app']?.getServerUrl(`ob/image/${profile.get('avatarHashes')?.get('small')}`)
      });

      cart.items?.forEach(item => {
        $.get(window['app']?.getServerUrl(`ob/listing/${item.listingHash}`)).then(listing => {
          let listingItem = listing.listing.item;
          item.title = listingItem.title;
          item.slug = listing.listing.slug;
          item.image = window['app']?.getServerUrl(`ob/image/${listingItem.images[0]?.small}`);
          item.price = listingItem.price;
          item.acceptedCurrencies = listing.listing.metadata.acceptedCurrencies;
        })
      });
    })
  })
  // tableData.value = products;
  loading.value = false;
}, 100);

//每个商品总价
const countRowPrice = computed(() => {
  return (row) => Number(row.price * row.quantity).toFixed(2);
});
//每个商店商品总价
const oneStoreTotalPrice = computed(() => (index) => {
  let list = selectors.value[index];
  if (!list) return 0;
  return { quantity: list.length, total: list.reduce((cur, next) => cur + next.price * next.quantity, 0) };
});
//购物车商品总数量
const cartNum = computed(() => {
  return tableData.value.reduce((cur, next) => cur + next.items.length, 0);
});

function goToStore(peerID) {
  window.location = `#${peerID}/store`;
}

function goToListing(vendorID, slug) {
  window.location = `#${vendorID}/store/${slug}`;
}

function handleSelectionChange(val, index) {
  selectors.value[index] = val;
}
//提交当前选中的商店商品
function pay(index) {
  const selection = table[index].store.states.selection;
  if (selection.value.length === 0) return ElMessage.warning('请选择商品');
  console.log(selection.value);
}
//修改头部样式
function headerRowStyle({ rowIndex }) {
  if (rowIndex === 0) return { background: 'transparent', color: '#000', fontSize: '18px' };
}
</script>

<style lang="scss" rowd>
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
    width: 32px;
    height: 32px;
    border-radius: 50%;
    flex-shrink: 0;
    margin-right: 10px;
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
    font-size: 16px;
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
    margin-right: 4px;
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
