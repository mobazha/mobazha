<template>
  <div class="page-container">
    <div class="page-main">
      <div class="page-head">
        <div class="page-head__left">
          <div class="page-head__name">
            Shopping Cart<span v-if="cartNum > 0">({{ cartNum }})</span>
          </div>
          <div class="clean-btn">Clear Cart</div>
        </div>
        <div class="page-head__right">
          <el-input
            v-model="params.keyword"
            placeholder="Search Orders"
            :prefix-icon="Search"
          />
        </div>
      </div>
      <el-table
        class="table-hc"
        :header-row-style="headerRowStyle"
        :data="[]"
        :height="38"
      >
        <el-table-column label="" width="48"></el-table-column>
        <el-table-column label="Title" width="400"></el-table-column>
        <el-table-column label="Type" width="160"></el-table-column>
        <el-table-column label="Price"></el-table-column>
        <el-table-column label="Quantity"></el-table-column>
        <el-table-column label="Total"></el-table-column>
      </el-table>
      <div class="card">
        <el-table
          :data="tableData"
          :cell-style="cellStyle"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="48" rowspan="2">
          </el-table-column>
          <el-table-column label="" width="400">
            <template #header>
              <div class="user">
                <img class="user-avatar" src="@/assets/img/avatar.png" />
                <span class="user-name">SBA preparedness outlet</span>
              </div>
            </template>
            <template #default="scoped">
              <div class="goods">
                <div class="goods-left">
                  <img class="goods-img" src="@/assets/img/exam.png" />
                </div>
                <div class="goods-right">
                  <div class="goods-name">
                    Vintage US Navy Pilot Crew Emergency Light FA-11(M) 6230 -
                    01 - 035 - 6077 (OCS)
                  </div>
                  <div class="goods-currency">
                    <img
                      class="currency-icon"
                      src="@/assets/img/currency/icon_1.png"
                    />
                    <img
                      class="currency-icon"
                      src="@/assets/img/currency/icon_2.png"
                    />
                    <img
                      class="currency-icon"
                      src="@/assets/img/currency/icon_3.png"
                    />
                  </div>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="" width="160">
            <template #default="scoped">
              <div class="sku">
                <div class="sku-item">
                  <div class="sku-label">Color</div>
                  <div class="sku-value">Green</div>
                </div>
                <div class="sku-item">
                  <div class="sku-label">Size</div>
                  <div class="sku-value">Small</div>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="">
            <template #default="scoped">
              {{ scoped.row.price }}
            </template>
          </el-table-column>
          <el-table-column label="">
            <template #default="scoped">
              <el-input class="input-number" v-model="num" />
              <!-- <el-input-number v-model="num" :min="1" /> -->
            </template>
          </el-table-column>
          <el-table-column label="">
            <template #default="scoped">
              <div class="row-price">
                {{ scoped.row.price }}
              </div>
            </template>
          </el-table-column>
        </el-table>
        <div class="footer">
          <div class="total">
            <div class="total-price">
              <span class="total-name">Total:</span>$318.24
            </div>
            <div class="count-price">Subtotal:$183.97</div>
            <div class="freight">Shipping & handling: Free</div>
          </div>
          <button class="pay-btn">Pay</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { Search } from "@element-plus/icons-vue";
import { reactive } from "vue";
const cartNum = ref(0);
const params = reactive({
  keyword: "",
});
const tableData = reactive([
  {
    price: "$49.70",
  },
  {
    price: "$49.70",
  },
  {
    price: "$49.70",
  },
  {
    price: "$49.70",
  },
]);
const num = ref(0);
const handleSelectionChange = (val) => {
  multipleSelection.value = val;
};
function headerRowStyle({ rowIndex }) {
  if (rowIndex === 0)
    return { background: "transparent", color: "#000", fontSize: "18px" };
}
function cellStyle() {
  return { "font-size": "14px", color: "#000" };
}
</script>

<style lang="scss" scoped>
@import '@/assets/scss/main.scss';

.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 31px 0 40px 0;
  margin-bottom: 30px;
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
  padding: 20px;
  border: 1px solid #e0e0e0;
  background: #fff;
}
.user {
  display: flex;
  align-items: center;
  &-avatar {
    width: 32px;
    height: 32rpx;
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
      content: ":";
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
  .pay-btn {
    position: relative;
    width: 120px;
    height: 48px;
    text-align: center;
    line-height: 48px;
    border: 1.5px solid #fff;
    background: linear-gradient(180deg, #80d769 0%, #5aae41 100%);
    box-shadow: 2px 2px 4px 0px rgba(0, 0, 0, 0.25);
    color: #fff;
    font-size: 24px;
    &:active {
      background: linear-gradient(180deg, #8fdc79 0%, #71b95b 100%);
    }
    &:hover:before {
      position: absolute;
      top: 50%;
      left: 50%;
      width: 100%;
      height: 100%;
      border: inherit;
      border-radius: inherit;
      transform: translate(-50%, -50%);
      opacity: 0.05;
      content: " ";
      background-color: #000;
      border-color: #000;
    }
  }
}
:deep() {
  .table-hc {
    margin: 0 20px;
    background: transparent;
    th.el-table__cell {
      background: transparent;
    }
  }
  .el-table__row {
    border: 1px solid red;
  }
}
</style>
