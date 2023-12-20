<template>
  <div class="external">
    <template v-if="!isHasAddress">
      <div class="external-desc">{{ ob.polyT('wallet.external.description', {coin: ob.polyT(`cryptoCurrencies.${mnCode}`, { _: mnCode })}) }}</div>
      <div class="external-box">
        <button v-if="!isAdd" class="btn-primary small" @click.stop="addAddress">{{ ob.polyT('wallet.external.addAddress') }}</button>
        <el-form v-if="isAdd" ref="formData" inline :model="formData" :rules="rules" label-width="0">
          <el-form-item prop="address">
            <el-input placeholder="Please input your Tether wallet address" v-model="formData.address" maxlength="200" size="large" />
          </el-form-item>
          <el-form-item>
            <button class="btn-primary small" @click.stop="onSubmit">{{ ob.polyT('wallet.external.add') }}</button>
          </el-form-item>
        </el-form>
      </div>
      <div class="tips">{{ ob.polyT('wallet.external.notice') }}</div>
    </template>
    <template v-else>
      <div class="external-desc">{{ ob.polyT('wallet.receiveMoney.title') }}</div>
      <div class="qrcode">
        <img class="qrcode-img" :src="qrUrl" />
      </div>
      <div class="code">0x7DAf18eqf59a6f2c5974740f13fEC4563207B92d <el-button class="copy-btn" link @click="copy">Edit Copy</el-button></div>
      <el-checkbox v-model="checked" :label="ob.polyT('wallet.external.enableLabel')" />
    </template>
  </div>
</template>

<script>
import qr from 'qr-encode';
import useClipboard from 'vue-clipboard3';
import { ElMessage } from 'element-plus';
export default {
  props: {
    code: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      qrUrl: '',
      checked: false,
      isHasAddress: false,
      isAdd: false,
      formData: {
        address: '',
      },
      rules: {
        address: [
          {
            required: true,
            validator(_rule, value, callback) {
              if (!value) return callback(new Error('Please input your Tether wallet address'));
              if (value !== '666') {
                return callback(new Error('Not a valid polygon wallet address'));
              }
              callback();
            },
            trigger: ['change', 'blur'],
          },
        ],
      },
    };
  },
  computed: {
    mnCode() {
      const ob = this.ob;

      return this.code && ob.crypto.ensureMainnetCode(this.code);
    },
  },
  methods: {
    addAddress() {
      this.isAdd = true;
    },
    copy() {
      const ob = this.ob;

      const { toClipboard } = useClipboard();
      toClipboard('0x7DAf18eqf59a6f2c5974740f13fEC4563207B92d');
      ElMessage.success(ob.polyT('copiedToClipboardShort'));
    },
    onSubmit() {
      this.$refs.formData.validate((valid) => {
        if (valid) {
          let qrUrl = qr('https://www.baidu.com', { type: 7, size: 5, level: 'M' });
          this.qrUrl = qrUrl;
          this.isHasAddress = true;
        }
      });
    },
  },
};
</script>

<style lang="scss" scoped>
.external {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  min-height: 320px;
  padding: 15px;
  box-sizing: border-box;
  &-desc {
    width: 50%;
    text-align: center;
    font-size: 14px;
  }
  &-box {
    margin: 40px 0;
  }
  .qrcode {
    width: 225px;
    height: 225px;
    margin: 20px 0;
    &-img {
      width: 100%;
      height: 100%;
    }
  }
  .tips {
    color: red;
    font-size: 13px;
  }
  .code {
    display: flex;
    align-items: center;
    font-size: 14px;
    margin-bottom: 10px;
  }
  .copy-btn {
    text-decoration: underline;
    margin-left: 10px;
  }
}

::v-deep() {
  .el-checkbox__input.is-checked .el-checkbox__inner {
    background-color: hsl(117, 66%, 41%);
    border-color: hsl(117, 66%, 41%);
  }
  .el-checkbox__input.is-checked + .el-checkbox__label {
    color: hsl(117, 66%, 41%);
  }
  .el-form-item:not(:last-child) {
    min-width: 240px;
  }
  .el-form-item {
    margin-right: 10px;
  }
  input {
    border: none !important;
    padding: 0 !important;
    line-height: var(--el-input-inner-height) !important;
  }
}
</style>
