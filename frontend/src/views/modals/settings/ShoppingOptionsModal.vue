<template>
  <el-dialog class="form-dialog" title="添加服务" v-model="visible" width="1080px" append-to-body :before-close="onCancel">
    <el-form ref="form" size="small" :model="formData" :rules="rules" label-width="0">
      <div class="form-head">
        <el-form-item label="模板" label-width="auto" prop="serviceType" :rules="rules.serviceType">
          <el-select @change="changeSelect" v-model="formData.serviceType" placeholder="请选择模板">
            <el-option v-for="item in options" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-button class="add-btn" link type="success" @click="doAdd">+添加</el-button>
      </div>
      <div class="tips" v-if="formData.serviceType">
        <span class="tips-btn">说明!</span>{{ serviceTypeTip }}
      </div>
      <el-table :data="formData.services" :border="true" row-class-name="form-table" cell-class-name="cell-form-table">
        <el-table-column label="服务" width="130">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.name`" :rules="rules.name">
              <el-input v-model="row.name" placeholder="例如标准，中通快递，隔日到" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="运送时间" width="110">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.estimatedDelivery`" :rules="rules.estimatedDelivery">
              <el-input v-model="row.estimatedDelivery" placeholder="例如5-7天" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="开始重量">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.startWeight`" :rules="rules.startWeight">
              <el-input v-model.number="row.startWeight" placeholder="0" disabled clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="结束重量">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.endWeight`" :rules="rules.endWeight">
              <el-input v-model.number="row.endWeight" placeholder="请输入" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <template v-if="formData.serviceType === 'FIRST_RENEWAL_FEE'">
          <el-table-column label="首重">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.firstWeight`" :rules="rules.firstWeight">
                <el-input v-model.number="row.firstWeight" placeholder="请输入" clearable maxlength="20" />
              </el-form-item>
            </template>
          </el-table-column>
          <el-table-column label="首重运费">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.firstFreight`" :rules="rules.firstFreight">
                <el-input v-model.number="row.firstFreight" placeholder="请输入" clearable maxlength="20" data-var-type="bignumber" />
              </el-form-item>
            </template>
          </el-table-column>
          <el-table-column label="续重单位重量">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.renewalUnitWeight`" :rules="rules.renewalUnitWeight">
                <el-input v-model.number="row.renewalUnitWeight" placeholder="请输入" clearable maxlength="20" />
              </el-form-item>
            </template>
          </el-table-column>
          <el-table-column label="单价">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.renewalUnitPrice`" :rules="rules.renewalUnitPrice">
                <el-input v-model.number="row.renewalUnitPrice" placeholder="请输入" clearable maxlength="20" data-var-type="bignumber" />
              </el-form-item>
            </template>
          </el-table-column>
        </template>
        <template v-else>
          <el-table-column label="费用">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.firstFreight`" :rules="rules.firstFreight">
                <el-input v-model.number="row.firstFreight" step="0.01" placeholder="请输入" clearable maxlength="20" data-var-type="bignumber" />
              </el-form-item>
            </template>
          </el-table-column>
        </template>
        
        <el-table-column label="挂号费">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.registrationFee`" :rules="rules.registrationFee">
              <el-input v-model.number="row.registrationFee" placeholder="请输入" clearable maxlength="20" />
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
import app from '../../../../backbone/app';
import bigNumber from 'bignumber.js';

const checkNonNegtiveVal = (rule, value, callback) => {
  if (value < 0) {
    callback(new Error(app.polyglot.t('currencyAmountErrors.greaterThanEqualZero')));
  } else {
    callback();
  }
};

const checkPositiveVal = (rule, value, callback) => {
  if (value <= 0) {
    callback(new Error(app.polyglot.t('currencyAmountErrors.greaterThanZero')));
  } else {
    callback();
  }
};

export default {
  props: {
    bb: Function,
  },
  computed: {
    serviceTypeTip() {
      if (this.formData.serviceType === 'FIRST_RENEWAL_FEE') {
        return '运费=首重费用+ (包裹重量-首重)/续重单位重量单价+挂号费';
      } else if (this.formData.serviceType === 'SAME_WEIGHT_SAME_FEE') {
        return '运费=费用+挂号费';
      }
      return '';
    },
  },
  data() {
    return {
      options: [
        { label: '按首重续费计算', value: 'FIRST_RENEWAL_FEE' },
        { label: '同重量段费用相同', value: 'SAME_WEIGHT_SAME_FEE' },
      ],
      visible: false,
      isSubmitIng: false,
      formData: {
        serviceType: '',
        services: [],
      },
      rules: {
        serviceType: [
          {
            required: true,
            message: '请选择模板',
            trigger: ['change', 'blur'],
          },
        ],
        name: [
          { required: true, message: '请输入服务',  trigger: ['change', 'blur'], },
        ],
        estimatedDelivery: [
          { required: true, message: '请输入运送时间', trigger: ['change', 'blur'], },
        ],
        startWeight: [
          { required: true, message: '请输入开始重量', trigger: ['change', 'blur'], },
        ],
        endWeight: [
          { required: true, message: '请输入结束重量', trigger: ['change', 'blur'], },
          { type: 'number', message: 'Input must be a number' },
          { validator: checkPositiveVal, trigger: ['change', 'blur'] }
        ],
        firstWeight: [
          { required: true, message: '请输入首重', trigger: ['change', 'blur'], },
          { type: 'number', message: 'Input must be a number' },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] }
        ],
        firstFreight: [
          { required: true, message: '请输入首重运费', trigger: ['change', 'blur'], },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] }
        ],
        renewalUnitWeight: [
          { required: true, message: '请输入续重单位重量', trigger: ['change', 'blur'], },
          { type: 'number', message: 'Input must be a number' },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] }
        ],
        renewalUnitPrice: [
          { required: true, message: '请输入单价', trigger: ['change', 'blur'], },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] }
        ],
        registrationFee: [
          { required: true, message: '请输入挂号费', trigger: ['change', 'blur'], },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] }
        ],
      },
    };
  },
  created() {
    this.loadData();
  },
  methods: {
    async open() {
      this.visible = true;
    },
    loadData () {
      if (!this.shippingOption) {
        throw new Error('Please provide a shippingOption model.');
      }

      this.initFormData();

      this.shippingOption.on('change', () => this.initFormData());
    },
    initFormData() {
      const optionData = this.shippingOption.toJSON();

      this.formData = {
        serviceType: optionData.serviceType,
        services: optionData.services,
      }
      if (optionData.services.length > 0) {
        this.formData.services = optionData.services;
      } else {
        this.formData.services = this.createEmptyService();
      }
    },
    createEmptyService() {
      return { name: '', estimatedDelivery: '', startWeight: '', endWeight: '', firstWeight: 0, firstFreight: '', renewalUnitWeight: '', renewalUnitPrice: '', registrationFee: '' };
    },
    onCancel() {
      this.visible = false;
    },
    changeSelect() {
      this.$refs.form.clearValidate();
    },
    doAdd() {
      this.formData.services.push(this.createEmptyService());
    },
    doDelete(index) {
      this.formData.services.splice(index, 1);
    },
    onConfirm() {
      this.$refs.form.validate((valid) => {
        if (valid) {
          const formData = this.formData;
          formData.services.forEach((service) => {
            service.firstFreight = bigNumber(service.firstFreight);
            service.renewalUnitPrice = bigNumber(service.renewalUnitPrice);
            service.registrationFee = bigNumber(service.registrationFee);
          })

          this.shippingOption.set(this.formData);

          this.visible = false;
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
