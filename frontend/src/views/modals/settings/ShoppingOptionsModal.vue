<template>
  <el-dialog class="form-dialog" title="添加服务" v-model="visible" width="1080px" append-to-body :before-close="onCancel">
    <el-form ref="formData" size="small" :model="formData" :rules="rules" label-width="0">
      <div class="form-head">
        <el-form-item label="模板" label-width="auto" prop="templateId" :rules="rules.templateId">
          <el-select @change="changeSelect" v-model="formData.templateId" placeholder="请选择模板">
            <el-option v-for="item in options" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-button class="add-btn" link type="success" @click="doAdd">+添加</el-button>
      </div>
      <div class="tips" v-if="formData.templateId">
        <span class="tips-btn">说明!</span>
        <template v-if="formData.templateId === '0'">运费=首重费用+ (包裹重量-首重)/续重单位重量单价+挂号费</template>
        <template v-if="formData.templateId === '1'">运费=费用+挂号费</template>
      </div>
      <el-table :data="formData.options" :border="true" row-class-name="form-table" cell-class-name="cell-form-table">
        <el-table-column label="服务" width="130">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.server`" :rules="rules.server">
              <el-input v-model="row.server" placeholder="例如标准，中通快递，隔日到" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="运送时间" width="110">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.deliveryTime`" :rules="rules.deliveryTime">
              <el-input v-model="row.deliveryTime" placeholder="例如5-7天" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="开始重量">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.startWeight`" :rules="rules.startWeight">
              <el-input v-model="row.startWeight" placeholder="请输入" clearable maxlength="20" type="number" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="结束重量">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.endWeight`" :rules="rules.endWeight">
              <el-input v-model="row.endWeight" placeholder="请输入" clearable maxlength="20" type="number" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column v-if="formData.templateId === '0'" label="价格（首重）">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.firstPrice`" :rules="rules.firstPrice">
              <el-input v-model="row.firstPrice" placeholder="请输入" clearable maxlength="20" type="number" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="首重运费">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.firstFreight`" :rules="rules.firstFreight">
              <el-input v-model="row.firstFreight" placeholder="请输入" clearable maxlength="20" type="number" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column v-if="formData.templateId === '0'" label="价格（续重）">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.renewalFee`" :rules="rules.renewalFee">
              <el-input v-model="row.renewalFee" placeholder="请输入" clearable maxlength="20" type="number" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="单价">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.price`" :rules="rules.price">
              <el-input v-model="row.price" placeholder="请输入" clearable maxlength="20" type="number" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="挂号费">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`options.${$index}.registrationFee`" :rules="rules.registrationFee">
              <el-input v-model="row.registrationFee" placeholder="请输入" clearable maxlength="20" type="number" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="90">
          <template v-slot="{ $index }">
            <el-button class="p0" size="small" type="danger" link @click="doDelete($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-form>
    <template #footer>
      <el-button size="small" @click="onCancel"> 取 消 </el-button>
      <el-button size="small" class="form-btn" type="primary" :disabled="isSubmitIng" :loading="isSubmitIng" @click="onConfirm">{{
        isSubmitIng ? '提交中...' : '确 定'
      }}</el-button>
    </template>
  </el-dialog>
</template>

<script>
export default {
  data() {
    return {
      options: [
        { label: '模板一', value: '0' },
        { label: '模板二', value: '1' },
      ],
      visible: false,
      isSubmitIng: false,
      formData: {
        templateId: '',
        options: [
          {
            server: '',
            deliveryTime: '',
            startWeight: '',
            endWeight: '',
            firstPrice: '',
            firstFreight: '',
            renewalFee: '',
            price: '',
            registrationFee: '',
          },
        ],
      },
      rules: {
        templateId: [
          {
            required: true,
            message: '请选择模板',
            trigger: ['change', 'blur'],
          },
        ],
        server: [
          {
            required: true,
            message: '请输入服务',
            trigger: ['change', 'blur'],
          },
        ],
        deliveryTime: [
          {
            required: true,
            message: '请输入运送时间',
            trigger: ['change', 'blur'],
          },
        ],
        startWeight: [
          {
            required: true,
            message: '请输入开始重量',
            trigger: ['change', 'blur'],
          },
        ],
        endWeight: [
          {
            required: true,
            message: '请输入结束重量',
            trigger: ['change', 'blur'],
          },
        ],
        firstPrice: [
          {
            required: true,
            message: '请输入首重价格',
            trigger: ['change', 'blur'],
          },
        ],
        firstFreight: [
          {
            required: true,
            message: '请输入首重运费',
            trigger: ['change', 'blur'],
          },
        ],
        renewalFee: [
          {
            required: true,
            message: '请输入续重价格',
            trigger: ['change', 'blur'],
          },
        ],
        price: [
          {
            required: true,
            message: '请输入单价',
            trigger: ['change', 'blur'],
          },
        ],
        registrationFee: [
          {
            required: true,
            message: '请输入挂号费',
            trigger: ['change', 'blur'],
          },
        ],
      },
    };
  },
  methods: {
    async open() {
      this.visible = true;
    },
    onCancel() {
      this.$refs.formData.resetFields();
      this.formData = {
        templateId: '',
        options: [
          { server: '', deliveryTime: '', startWeight: '', endWeight: '', firstPrice: '', firstFreight: '', renewalFee: '', price: '', registrationFee: '' },
        ],
      };
      this.visible = false;
    },
    changeSelect() {
      this.$refs.formData.clearValidate();
    },
    doAdd() {
      this.formData.options.push({
        server: '',
        deliveryTime: '',
        startWeight: '',
        endWeight: '',
        firstPrice: '',
        firstFreight: '',
        renewalFee: '',
        price: '',
        registrationFee: '',
      });
    },
    doDelete(index) {
      this.formData.options.splice(index, 1);
    },
    onConfirm() {
      this.$refs.formData.validate(async (valid) => {
        if (valid) {
          this.visible = false;
          this.$emit('getExpressInfo', JSON.parse(JSON.stringify(this.formData)));
        }
      });
    },
  },
};
</script>

<style lang="scss" scoped>
.form-dialog {
  .form-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .add-btn {
    font-size: 16px;
  }
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
::v-deep() {
  .form-table.el-form-item__content {
    margin-left: 0 !important;
  }
  .form-table .is-error .el-form-item__content {
    margin-bottom: 0;
  }
  .cell-form-table {
    vertical-align: top !important;
  }

  input {
    border: none !important;
    padding: 0 !important;
    line-height: var(--el-input-inner-height) !important;
  }
  .el-input {
    --el-input-border-radius: 0;
    --el-input-hover-border-color: #999;
    --el-input-focus-border-color: #999;
  }
}
.el-button {
  border-radius: 2px;
}
.p0 {
  padding: 0;
}
</style>
