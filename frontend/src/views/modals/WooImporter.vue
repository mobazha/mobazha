<template>
  <div class="modal modalScrollPage">
    <BaseModal @close="close">
      <template v-slot:component>
        <div class="topControls flex">
          <a class="btn clrP clrBr " @click="onCopyClick">{{ ob.polyT('debugLog.copyToClipboard') }}</a>
          <div class="js-copiedConfirm pad tx6 clrT2 hide">{{ ob.polyT('copiedToClipboard') }}</div>
        </div>

        <div class="contentBox padMd clrP clrBr clrSh3">
          <div class="js-debugLog preWrap tx6">{{ ob.debugLog }}</div>
        </div>

        <el-table
          ref="multipleTableRef"
          :data="tableData"
          style="width: 100%"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="55" />
          <el-table-column label="Date" width="120">
            <template #default="scope">{{ scope.row.date }}</template>
          </el-table-column>
          <el-table-column property="name" label="Name" width="120" />
          <el-table-column property="address" label="Address" show-overflow-tooltip />
        </el-table>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../backbone/app.js';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      debugLog: app.serverLog,
      maxDebugLines: 1000,

      multipleSelection: [],

      tableData: [
        {
          date: '2016-05-03',
          name: 'Tom',
          address: 'No. 189, Grove St, Los Angeles',
        },
        {
          date: '2016-05-02',
          name: 'Tom',
          address: 'No. 189, Grove St, Los Angeles',
        },
        {
          date: '2016-05-04',
          name: 'Tom',
          address: 'No. 189, Grove St, Los Angeles',
        },
        {
          date: '2016-05-01',
          name: 'Tom',
          address: 'No. 189, Grove St, Los Angeles',
        },
        {
          date: '2016-05-08',
          name: 'Tom',
          address: 'No. 189, Grove St, Los Angeles',
        },
        {
          date: '2016-05-06',
          name: 'Tom',
          address: 'No. 189, Grove St, Los Angeles',
        },
        {
          date: '2016-05-07',
          name: 'Tom',
          address: 'No. 189, Grove St, Los Angeles',
        },
      ]
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  unmounted() {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        debugLog: this.debugLog,
      };
    }
  },
  methods: {
    loadData (options = {}) {

    },

    onCopyClick () {
    },

    handleSelectionChange(val) {
      multipleSelection = val
    }
  }
}
</script>
<style lang="scss" scoped></style>
