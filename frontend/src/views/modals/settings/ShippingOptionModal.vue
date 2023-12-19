<template>
  <el-dialog class="form-dialog" :title="ob.polyT('settings.storeTab.shippingOptions.modal.addService')" v-model="visible" width="1080px" append-to-body :before-close="onCancel">
    <el-form ref="form" size="small" :model="formData" :rules="rules" label-width="0">
      <div class="form-head">
        <div class="form-head__select">
          <el-form-item :label="ob.polyT('settings.storeTab.shippingOptions.modal.template')" label-width="auto" prop="serviceType" :rules="rules.serviceType">
            <el-select @change="changeSelect" v-model="formData.serviceType" :placeholder="ob.polyT('settings.storeTab.shippingOptions.modal.selectTemplate')">
              <el-option v-for="item in options" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-button class="add-btn" link type="success" @click="doAdd">{{ ob.polyT('settings.storeTab.shippingOptions.modal.add') }}</el-button>
        </div>
        <div class="tips" v-if="formData.serviceType"><span class="tips-btn">{{ ob.polyT('settings.storeTab.shippingOptions.services.notice') }}</span>{{ serviceTypeTip }}</div>
      </div>
      <el-table :data="formData.services" :border="true" row-class-name="form-table" cell-class-name="cell-form-table">
        <el-table-column :label="ob.polyT('settings.storeTab.shippingOptions.services.nameLabel')" width="130">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.name`" :rules="rules.name">
              <el-input v-model="row.name" :placeholder="ob.polyT('settings.storeTab.shippingOptions.services.namePlaceholder')" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column :label="ob.polyT('settings.storeTab.shippingOptions.services.estimatedDeliveryLabel')" width="110">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.estimatedDelivery`" :rules="rules.estimatedDelivery">
              <el-input v-model="row.estimatedDelivery" :placeholder="ob.polyT('settings.storeTab.shippingOptions.services.estimatedDeliveryPlaceholder')" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column :label="`${ob.polyT('settings.storeTab.shippingOptions.services.startWeight')}(g)`">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.startWeight`" :rules="rules.startWeight">
              <el-input v-model.number="row.startWeight" placeholder="0" disabled clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column :label="`${ob.polyT('settings.storeTab.shippingOptions.services.endWeight')}(g)`">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.endWeight`" :rules="rules.endWeight">
              <el-input v-model.number="row.endWeight" placeholder="0" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <template v-if="formData.serviceType === 'FIRST_RENEWAL_FEE'">
          <el-table-column :label="`${ob.polyT('settings.storeTab.shippingOptions.services.firstWeight')}(g)`">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.firstWeight`" :rules="rules.firstWeight">
                <el-input v-model.number="row.firstWeight" placeholder="0" clearable maxlength="20" />
              </el-form-item>
            </template>
          </el-table-column>
          <el-table-column :label="ob.polyT('settings.storeTab.shippingOptions.services.firstFreight')">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.firstFreight`" :rules="rules.firstFreight">
                <el-input v-model.number="row.firstFreight" step="0.01" :placeholder="ob.polyT('settings.storeTab.shippingOptions.services.pricePlaceholder')" clearable maxlength="20" data-var-type="bignumber" />
              </el-form-item>
            </template>
          </el-table-column>
          <el-table-column :label="`${ob.polyT('settings.storeTab.shippingOptions.services.renewalUnitWeight')}(g)`">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.renewalUnitWeight`" :rules="rules.renewalUnitWeight">
                <el-input v-model.number="row.renewalUnitWeight" placeholder="0" clearable maxlength="20" />
              </el-form-item>
            </template>
          </el-table-column>
          <el-table-column :label="ob.polyT('settings.storeTab.shippingOptions.services.renewalUnitPrice')">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.renewalUnitPrice`" :rules="rules.renewalUnitPrice">
                <el-input v-model.number="row.renewalUnitPrice" step="0.01" :placeholder="ob.polyT('settings.storeTab.shippingOptions.services.pricePlaceholder')" clearable maxlength="20" data-var-type="bignumber" />
              </el-form-item>
            </template>
          </el-table-column>
        </template>
        <template v-else>
          <el-table-column :label="ob.polyT('settings.storeTab.shippingOptions.services.fee')">
            <template v-slot="{ row, $index }">
              <el-form-item :prop="`services.${$index}.firstFreight`" :rules="rules.firstFreight">
                <el-input v-model.number="row.firstFreight" step="0.01" :placeholder="ob.polyT('settings.storeTab.shippingOptions.services.pricePlaceholder')" clearable maxlength="20" data-var-type="bignumber" />
              </el-form-item>
            </template>
          </el-table-column>
        </template>

        <el-table-column :label="ob.polyT('settings.storeTab.shippingOptions.services.registrationFee')">
          <template v-slot="{ row, $index }">
            <el-form-item :prop="`services.${$index}.registrationFee`" :rules="rules.registrationFee">
              <el-input v-model.number="row.registrationFee" step="0.01" :placeholder="ob.polyT('settings.storeTab.shippingOptions.services.pricePlaceholder')" clearable maxlength="20" />
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column :label="ob.polyT('settings.storeTab.shippingOptions.services.action')" fixed="right" width="90">
          <template v-slot="{ $index }">
            <el-button class="p0" size="small" type="danger" link @click="doDelete($index)">{{ ob.polyT('settings.storeTab.shippingOptions.services.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-form>
    <template #footer>
      <el-button size="small" @click="onCancel"> {{ ob.polyT('settings.storeTab.shippingOptions.services.cancel') }} </el-button>
      <el-button size="small" class="form-btn" type="primary" :disabled="isSubmitting" :loading="isSubmitting" @click="onConfirm">{{
        isSubmitting ? ob.polyT('settings.storeTab.shippingOptions.services.submitting') : ob.polyT('settings.storeTab.shippingOptions.services.confirm')
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
        return app.polyglot.t('settings.storeTab.shippingOptions.services.firstRenewalExplanation');
      } else if (this.formData.serviceType === 'SAME_WEIGHT_SAME_FEE') {
        return app.polyglot.t('settings.storeTab.shippingOptions.services.sameWeightExplanation');
      }
      return '';
    },
  },
  data() {
    const pleaseInput = app.polyglot.t('settings.storeTab.shippingOptions.modal.pleaseInput');
    return {
      options: [
        { label: app.polyglot.t('settings.storeTab.shippingOptions.services.firstRenewalTemplate'), value: 'FIRST_RENEWAL_FEE' },
        { label: app.polyglot.t('settings.storeTab.shippingOptions.services.sameWeightTemplate'), value: 'SAME_WEIGHT_SAME_FEE' },
      ],
      visible: false,
      isSubmitting: false,
      formData: {
        serviceType: '',
        services: [],
      },
      rules: {
        serviceType: [
          {
            required: true,
            message: app.polyglot.t('settings.storeTab.shippingOptions.modal.selectTemplate'),
            trigger: ['change', 'blur'],
          },
        ],
        name: [
          { required: true, message: pleaseInput, trigger: ['change', 'blur'] },
          { validator: this.checkNameDuplicate, trigger: ['change'] },
        ],
        estimatedDelivery: [{ required: true, message: 'pleaseInput', trigger: ['change', 'blur'] }],
        startWeight: [{ required: true, message: 'pleaseInput', trigger: ['change', 'blur'] }],
        endWeight: [
          { required: true, message: pleaseInput, trigger: ['change', 'blur'] },
          { type: 'number', message: 'Input must be a number' },
          { validator: checkPositiveVal, trigger: ['change', 'blur'] },
        ],
        firstWeight: [
          { required: true, message: pleaseInput, trigger: ['change', 'blur'] },
          { type: 'number', message: 'Input must be a number' },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] },
        ],
        firstFreight: [
          { required: true, message: pleaseInput, trigger: ['change', 'blur'] },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] },
        ],
        renewalUnitWeight: [
          { required: true, message: pleaseInput, trigger: ['change', 'blur'] },
          { type: 'number', message: 'Input must be a number' },
          { validator: checkPositiveVal, trigger: ['change', 'blur'] },
        ],
        renewalUnitPrice: [
          { required: true, message: pleaseInput, trigger: ['change', 'blur'] },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] },
        ],
        registrationFee: [
          { required: true, message: pleaseInput, trigger: ['change', 'blur'] },
          { validator: checkNonNegtiveVal, trigger: ['change', 'blur'] },
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
    loadData() {
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
      };
      if (optionData.services && optionData.services.length > 0) {
        this.formData.services = optionData.services;
      } else {
        this.formData.services = [this.createEmptyService()];
      }
    },
    createEmptyService() {
      return {
        name: '',
        estimatedDelivery: '',
        startWeight: 0,
        endWeight: 0,
        firstWeight: 0,
        firstFreight: 0,
        renewalUnitWeight: 0,
        renewalUnitPrice: 0,
        registrationFee: 0,
      };
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
          });

          this.shippingOption.set(this.formData);

          this.$emit('shippingOptionUpdated');

          this.visible = false;
        }
      });
    },
    checkNameDuplicate(rule, value, callback) {
      for (let i = 0; i < this.formData.services.length; i++) {
        if (`services.${i}.name` === rule.field) {
          // skip self check
          continue;
        }

        if (this.formData.services[i].name === value) {
          callback(new Error(app.polyglot.t('settings.storeTab.shippingOptions.modal.serviceNameDuplicate')));
          return;
        }
      }
      callback();
    },
  },
};
</script>

<style lang="scss" scoped>
.form-dialog {
  .form-head {
    margin-bottom: 10px;
    &__select {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 8px;
    }
  }

  .add-btn {
    font-size: 16px;
  }
}
.tips {
  display: flex;
  align-items: center;
  color: #999;
  &-btn {
    padding: 2px 14px;
    background: green;
    color: #fff;
    margin-right: 10px;
  }
}
::v-deep() {
  .cell-form-table {
    vertical-align: top !important;
  }
  .el-form-item {
    margin-bottom: 0;
  }
  .el-form-item__error {
    position: static;
    line-height: 1.2;
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
