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
          <div class="search">
            <el-input class="search-input" v-model="params.keyword" placeholder="Search Orders" :prefix-icon="Search" />
          </div>
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
              <el-table ref="table" class="table-hearder-one" :data="item.children" @selection-change="handleSelectionChange($event, index)">
                <el-table-column type="selection" width="48" rowspan="2"> </el-table-column>
                <el-table-column width="400">
                  <template #header>
                    <div class="user">
                      <img class="user-avatar" :src="item.avatar" />
                      <span class="user-name">{{ item.name }}</span>
                    </div>
                  </template>
                  <template v-slot="{ row }">
                    <div class="goods">
                      <div class="goods-left">
                        <img class="goods-img" :src="row.url" />
                      </div>
                      <div class="goods-right">
                        <div class="goods-name">{{ row.name }}</div>
                        <div class="goods-currency">
                          <img class="currency-icon" src="@/assets/img/currency/icon_1.png" />
                          <img class="currency-icon" src="@/assets/img/currency/icon_2.png" />
                          <img class="currency-icon" src="@/assets/img/currency/icon_3.png" />
                        </div>
                      </div>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column width="160">
                  <template v-slot="{ row }">
                    <div class="sku">
                      <div class="sku-item" v-for="(val, key) in row.sku" :key="key">
                        <div class="sku-label">{{ val.label }}</div>
                        <div class="sku-value">{{ val.value }}</div>
                      </div>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="price"></el-table-column>
                <el-table-column>
                  <template v-slot="{ row }">
                    <el-input class="input-number" v-model="row.num" />
                  </template>
                </el-table-column>
                <el-table-column>
                  <template v-slot="{ row }">
                    {{ countRowPrice(row) }}
                  </template>
                </el-table-column>
              </el-table>
              <div class="footer" v-if="oneStoreTotalPrice(index).num > 0">
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
import Empty from './Empty.vue';
import { products } from './products.js';

const params = reactive({ keyword: '' });
const tableData = ref([]);
const table = reactive(null);
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
  tableData.value = products;
  loading.value = false;
}, 1000);
//每个商品总价
const countRowPrice = computed(() => {
  return (row) => Number(row.price * row.num).toFixed(2);
});
//每个商店商品总价
const oneStoreTotalPrice = computed(() => (index) => {
  let list = selectors.value[index];
  if (!list) return 0;
  return { num: list.length, total: list.reduce((cur, next) => cur + next.price * next.num, 0) };
});
//购物车商品总数量
const cartNum = computed(() => {
  return tableData.value.reduce((cur, next) => cur + next.children.length, 0);
});

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
    font-size: 16px;
    color: #000000;
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
  &-name {
    color: #000;
    font-size: 14px;
    line-height: 20px;
    margin-bottom: 8px;
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
  &-label {
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
