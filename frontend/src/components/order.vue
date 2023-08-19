<template>
  <div class="page-container">
    <div class="page-main">
      <div class="page-head">
        <div class="page-head__left">
          <div class="page-head__name">Orders</div>
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
              <el-table-column label="Listing" width="350"></el-table-column>
              <el-table-column label="Type" width="120"></el-table-column>
              <el-table-column label="Price" width="100"></el-table-column>
              <el-table-column label="Quantity" width="100"></el-table-column>
              <el-table-column label="Total"></el-table-column>
              <el-table-column label="Status"></el-table-column>
            </el-table>
          </div>
          <div class="card">
            <div class="card-item" v-for="(item, index) in tableData" :key="index">
              <el-table :data="item.children" :span-method="spanMethod" :header-cell-style="headerCellStyle" :cell-style="cellStyle">
                <el-table-column>
                  <template #header>
                    <div class="table-head">
                      <div class="cell">
                        <div class="cell-item">
                          <div class="cell-label">Date:</div>
                          <div class="cell-value">8/7/2023 12:29 PM</div>
                        </div>
                        <div class="cell-item">
                          <div class="cell-label">Order:</div>
                          <div class="cell-value">QmUtkKDadfdf12124</div>
                        </div>
                        <div class="cell-item">
                          <div class="cell-label">Vedor:</div>
                          <div class="cell-value"><img class="user-avatar" src="@/assets/img/avatar.png" />SBA preparedness outlet</div>
                        </div>
                      </div>
                      <img src="@/assets/img/delete.png" class="delete-icon" @click="doDelete(index)" />
                    </div>
                  </template>
                  <template #default>
                    <el-table-column width="350">
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
                    <el-table-column width="120">
                      <template v-slot="{ row }">
                        <div class="sku">
                          <div class="sku-item" v-for="(val, key) in row.sku" :key="key">
                            <div class="sku-label">{{ val.label }}</div>
                            <div class="sku-value">{{ val.value }}</div>
                          </div>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column width="100" prop="price"></el-table-column>
                    <el-table-column width="50" prop="num"></el-table-column>
                    <el-table-column>
                      <template v-slot="{ $index, row }">
                        <div v-if="$index === 0">
                          <div class="row-price">{{ row.price }}</div>
                          <div class="freight">Shipping & handling:</div>
                          <div class="freight-price">Free</div>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column>
                      <template v-slot="{ $index, row }">
                        <div v-if="$index === 0">
                          <template v-if="row.status === 0">Awaiting Payment</template>
                          <template v-if="row.status === 1">Completed</template>
                        </div>
                      </template>
                    </el-table-column>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </template>
      </div>

      <empty v-if="tableData.length === 0 && !loading" :emptyInfo="emptyInfo" />
    </div>
  </div>
</template>

<script setup>
import empty from '@/components/empty';
import { Search } from '@element-plus/icons-vue';
import { reactive } from 'vue';
import { products } from './products.js';
import { ElMessage, ElMessageBox } from 'element-plus';

const params = reactive({ keyword: '' });
const tableData = ref([]);
const loading = ref(true);
const emptyInfo = ref({
  type: 'order',
  icon: new URL(`@/assets/img/empty/order.png`, import.meta.url).href,
  name: "You don't have any items",
  desc: "Go ahead, find something you'll love.",
  btn: 'Shop Popular Products',
});

setTimeout(() => {
  tableData.value = products;
  loading.value = false;
}, 1000);

function doDelete(index) {
  ElMessageBox.confirm('确定删除该订单吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action) => {
      if (action === 'confirm') {
        tableData.value.splice(index, 1);
        ElMessage({ type: 'success', message: '已删除' });
      }
    },
  });
}

function headerRowStyle({ rowIndex }) {
  if (rowIndex === 0) return { background: 'transparent', color: '#000', fontSize: '18px' };
}
function spanMethod({ rowIndex, columnIndex }) {
  if (rowIndex === 0) {
    if (columnIndex === 4 || columnIndex === 5) {
      return { rowspan: 2, colspan: 1 };
    }
    if (columnIndex === 5) {
      return { rowspan: 2, colspan: 1 };
    }
  }
}
function headerCellStyle({ columnIndex, rowIndex }) {
  if (rowIndex === 1) return { display: 'none' };
}
function cellStyle({ columnIndex, rowIndex }) {
  if (columnIndex === 3 || columnIndex === 4) return { 'border-right': '1px solid #e0e0e0' };
}
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
  line-height: 24px;
  letter-spacing: 0.14px;
  text-align: center;
}
.freight {
  color: #000;
  text-align: center;
  font-size: 13px;
  line-height: 16px; /* 123.077% */
  &-price {
    @extend .freight;
  }
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
.table-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  .cell {
    display: flex;
    align-items: center;
    &-item {
      display: flex;
      align-items: center;
      margin-right: 20px;
    }
    &-label {
      color: rgba(0, 0, 0, 0.5);
      font-size: 16px;
      line-height: normal;
      margin-right: 5px;
    }
    &-value {
      display: flex;
      align-items: center;
      flex: 1;
      color: #000;
      font-size: 16px;
      line-height: normal;
    }
  }
  .delete-icon {
    width: 20px;
    height: 20px;
  }
}
</style>
