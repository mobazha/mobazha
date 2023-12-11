<template>
  <div class="template-name" v-if="templateName">{{ templateName }}</div>
  <div class="tips" v-if="formData.serviceType">
    <span class="tips-btn">说明!</span>{{ serviceTypeTip }}
  </div>
  <table class="table" width="100%" border="1" cellpadding="0" cellspacing="0">
    <tr>
      <th>服务</th>
      <th>运送时间</th>
      <th>重量范围</th>
      <template v-if="formData.serviceType === 'FIRST_RENEWAL_FEE'">
        <th>首重/首重费用</th>
        <th>续重单位重量/单价</th>
      </template>
      <template v-else>
        <th>费用</th>
      </template>
      <th>挂号费</th>
    </tr>
    <tbody>
      <tr v-for="(item, index) in formData.services" :key="index">
        <td>{{ item.name }}</td>
        <td>{{ item.estimatedDelivery }}</td>
        <td>{{ `${item.startWeight}g ~ ${item.endWeight}g` }}</td>
        <template v-if="formData.serviceType === 'FIRST_RENEWAL_FEE'">
          <td>{{ `${item.firstWeight}g / ${item.firstFreight}` }}</td>
          <td>{{ `${item.renewalUnitWeight}g / ${item.renewalUnitPrice}` }}</td>
        </template>
        <template v-else>
          <td>{{ item.firstFreight }}</td>
        </template>
        <td>{{ item.registrationFee }}</td>
      </tr>
    </tbody>
  </table>
  <!-- <el-table :data="data.options" :border="true" scrollbar-always-on>
    <el-table-column label="服务" prop="service" show-overflow-tooltip />
    <el-table-column label="运送时间" prop="estimatedDelivery" show-overflow-tooltip />
    <el-table-column label="开始重量" prop="startWeight" show-overflow-tooltip />
    <el-table-column label="结束重量" prop="endWeight" show-overflow-tooltip />
    <el-table-column label="价格（首重）" prop="firstPrice" show-overflow-tooltip />
    <el-table-column label="首重运费" prop="firstFreight" show-overflow-tooltip />
    <el-table-column label="价格（续重）" prop="renewalFee" show-overflow-tooltip />
    <el-table-column label="单价" prop="renewalUnitPrice" show-overflow-tooltip />
    <el-table-column label="挂号费" prop="registrationFee" show-overflow-tooltip />
  </el-table> -->
</template>
<script>
export default {
  props: {
    bb: Function,
  },
  data() {
    return {
      formData: {
        serviceType: 'FIRST_RENEWAL_FEE',
        services: [],
      },
      options: [
        { label: '按首重续费计算', value: 'FIRST_RENEWAL_FEE' },
        { label: '同重量段费用相同', value: 'SAME_WEIGHT_SAME_FEE' },
      ],
    };
  },
  created () {
    this.loadData();
  },
  computed: {
    templateName() {
      if (!this.formData.serviceType) return '';
      return this.options.find((item) => item.value === this.formData.serviceType)?.label ?? '';
    },
    serviceTypeTip() {
      if (this.formData.serviceType === 'FIRST_RENEWAL_FEE') {
        return '运费=首重费用+ (包裹重量-首重)/续重单位重量单价+挂号费';
      } else if (this.formData.serviceType === 'SAME_WEIGHT_SAME_FEE') {
        return '运费=费用+挂号费';
      }
      return '';
    },
  },
  methods: {
    loadData () {
      if (!this.shippingOption) {
        throw new Error('Please provide a shippingOption model.');
      }

      this.initFormData();

      this.shippingOption.on('change', () => this.initFormData());
      this.shippingOption.get('services').on('change', () => this.initFormData());
    },

    initFormData() {
      const optionData = this.shippingOption.toJSON();

      this.formData = {
        serviceType: optionData.serviceType,
        services: optionData.services,
      }
    }
  }
};
</script>
<style lang="scss" scoped>
.template-name {
  height: 40px;
  line-height: 40px;
  font-size: 14px;
  font-weight: bold;
}
.tips {
  display: flex;
  align-items: center;
  color: #999;
  margin-bottom: 10px;
  &-btn {
    padding: 2px 14px;
    background: green;
    color: #fff;
    margin-right: 10px;
  }
}
.table {
  border-collapse: collapse;
  border: 1px solid #dadbdd;
  th {
    font-size: 13px;
  }
  th,
  td {
    padding: 5px 5px;
    text-align: left;
    box-sizing: border-box;
  }
  td {
    font-weight: 400;
    font-size: 12px;
  }
  .tb-bg {
    width: 140px;
    white-space: nowrap;
    padding: 5px 5px;
    background: #f2f3f7;
    font-size: 14px;
    font-weight: bold;
    border: 1px solid #dadbdd;
  }
  td {
    min-width: 60px;
  }
}
</style>
