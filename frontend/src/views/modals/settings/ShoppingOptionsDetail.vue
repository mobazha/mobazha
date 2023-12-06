<template>
  <div class="template-name" v-if="templateName">{{ templateName }}</div>
  <table class="table" width="100%" border="1" cellpadding="0" cellspacing="0">
    <tr>
      <th>服务</th>
      <th>运送时间</th>
      <th>开始重量</th>
      <th>结束重量</th>
      <th v-if="data.templateId === '0'">价格（首重）</th>
      <th>首重运费</th>
      <th v-if="data.templateId === '0'">价格（续重）</th>
      <th>单价</th>
      <th>挂号费</th>
    </tr>
    <tbody>
      <tr v-for="(item, index) in data.options" :key="index">
        <td>{{ item.service }}</td>
        <td>{{ item.deliveryTime }}</td>
        <td>{{ item.startWeight }}</td>
        <td>{{ item.endWeight }}</td>
        <td v-if="data.templateId === '0'">{{ item.firstPrice }}</td>
        <td>{{ item.firstFreight }}</td>
        <td v-if="data.templateId === '0'">{{ item.renewalFee }}</td>
        <td>{{ item.price }}</td>
        <td>{{ item.registrationFee }}</td>
      </tr>
    </tbody>
  </table>
  <!-- <el-table :data="data.options" :border="true" scrollbar-always-on>
    <el-table-column label="服务" prop="service" show-overflow-tooltip />
    <el-table-column label="运送时间" prop="deliveryTime" show-overflow-tooltip />
    <el-table-column label="开始重量" prop="startWeight" show-overflow-tooltip />
    <el-table-column label="结束重量" prop="endWeight" show-overflow-tooltip />
    <el-table-column label="价格（首重）" prop="firstPrice" show-overflow-tooltip />
    <el-table-column label="首重运费" prop="firstFreight" show-overflow-tooltip />
    <el-table-column label="价格（续重）" prop="renewalFee" show-overflow-tooltip />
    <el-table-column label="单价" prop="price" show-overflow-tooltip />
    <el-table-column label="挂号费" prop="registrationFee" show-overflow-tooltip />
  </el-table> -->
</template>
<script>
export default {
  data() {
    return {
      options: [
        { label: '模板一', value: '0' },
        { label: '模板二', value: '1' },
      ],
    };
  },
  props: {
    data: {
      type: Object,
      default: () => ({ templateId: '', options: [] }),
    },
  },
  computed: {
    templateName() {
      if (!this.data.templateId) return '';
      return this.options.find((item) => item.value === this.data.templateId)?.label ?? '';
    },
  },
};
</script>
<style lang="scss" scoped>
.template-name {
  height: 40px;
  line-height: 40px;
  font-size: 14px;
  font-weight: bold;
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
