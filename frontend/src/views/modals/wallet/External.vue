<template>
  <div class="external">
    <template v-if="editMode">
      <div class="external-desc">{{ ob.polyT('wallet.external.description', {coin: coinName}) }}</div>
      <div class="external-box">
        <el-button v-if="blankScreen" class="btn-primary small" @click.stop="beginAdd">{{ ob.polyT('wallet.external.addAddress') }}</el-button>
        <el-form v-if="!blankScreen" ref="formData" inline :model="formData" @validate="validateHandler" :rules="rules" label-width="0">
          <el-form-item prop="address">
            <el-input :placeholder="ob.polyT('wallet.external.inputPlaceHolder', {coin: coinName})" v-model="formData.address" maxlength="200" size="large" />
          </el-form-item>
          <el-form-item>
            <el-button class="btn-primary small" :disabled="!formValidity.address" @click.stop="onSubmit">{{ ob.polyT('wallet.external.add') }}</el-button>
          </el-form-item>
        </el-form>
      </div>
      <div class="tips">{{ ob.polyT('wallet.external.notice', {coin: coinName}) }}</div>
    </template>
    <template v-else>
      <div class="external-desc">{{ ob.polyT('wallet.receiveMoney.title') }}</div>
      <div class="qrcode">
        <img class="qrcode-img" :src="qrDataUri" />
      </div>
      <div class="code">{{ formData.address }} <el-button class="copy-edit-btn" link @click="onEdit">{{ ob.polyT('wallet.external.edit') }}</el-button><el-button class="copy-edit-btn" link @click="onCopy">{{ ob.polyT('wallet.external.copy') }}</el-button></div>
      <el-checkbox v-model="checked" @change="onChecked" :label="ob.polyT('wallet.external.enableLabel')" />
    </template>
  </div>
</template>

<script>
import qr from 'qr-encode';
import useClipboard from 'vue-clipboard3';
import { ElMessage } from 'element-plus';
import WAValidator from 'multicoin-address-validator';
import app from '../../../../backbone/app.js';
import { TIP_ADDRESSES, getCurrencyByCode } from '../../../../backbone/data/walletCurrencies.js';
export default {
  props: {
    coinType: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      editMode: false,
      blankScreen: true,

      lastAddress: '',
      checked: false,
      externalPaymentAddresses: {},
      formData: {
        address: '',
      },
      formValidity: {
        address: false,
      },
      rules: {
        address: [
          {
            required: true,
            validator: this.validateInputAddress,
            trigger: ['change', 'blur'],
          },
        ],
      },
    };
  },
  computed: {
    mnCode() {
      return this.coinType && this.ob.crypto.ensureMainnetCode(this.coinType);
    },
    coinName() {
      return this.ob.polyT(`cryptoCurrencies.${this.mnCode}`, { _: this.mnCode });
    },
    qrDataUri () {
      // defaulting to an empty image - needed for proper spacing
      // when the spinner is showing
      let qrDataUri = 'data:image/gif;base64,R0lGODlhAQABAAAAACw=';
      let walletCur;

      try {
        walletCur = getCurrencyByCode(this.coinType);
      } catch (e) {
        // pass
      }

      if (this.lastAddress && walletCur) {
        qrDataUri = qr(walletCur.qrCodeText(this.lastAddress), { type: 7, size: 5, level: 'M' });
      }
      return qrDataUri;
    }
  },
  created() {
    this.initEventChain();

    this.loadData();
  },
  methods: {
    validateHandler(propName, isValid) {
      this.formValidity[propName] = isValid;
    },

    validateInputAddress(_rule, value, callback) {
      if (!value) return callback(new Error(app.polyglot.t('wallet.external.inputPlaceHolder')));
      if (!WAValidator.validate(value, this.coinType)) {
        return callback(new Error(app.polyglot.t('wallet.external.invalidAddress', { coin: this.coinName, addr: TIP_ADDRESSES[this.coinType] })));
      }
      callback();
    },

    loadData() {
      this.settings = app.settings.clone();

      // Sync our clone with any changes made to the global settings model.
      this.listenTo(app.settings, 'someChange', (md, sOpts) => this.settings.set(sOpts.setAttrs));

      // Sync the global settings model with any changes we save via our clone.
      this.listenTo(this.settings, 'sync', (md, resp, sOpts) => app.settings.set(this.settings.toJSON(sOpts.attrs)));

      this.externalPaymentAddresses = this.settings.get('externalPaymentAddresses') || {};
      this.lastAddress = this.externalPaymentAddresses[this.coinType]?.address;
      this.checked = this.externalPaymentAddresses[this.coinType]?.enable;
      this.formData.address = this.lastAddress;

      this.editMode = !this.lastAddress;
      this.blankScreen = !this.lastAddress;
    },

    beginAdd() {
      this.blankScreen = false;
    },
    onEdit() {
      this.editMode = true;
    },
    onCopy() {
      const ob = this.ob;

      const { toClipboard } = useClipboard();
      toClipboard(this.lastAddress);
      ElMessage.success(ob.polyT('copiedToClipboardShort'));
    },
    onSubmit() {
      this.save();
    },

    onChecked() {
      this.save(true);
    },

    save (updateCheck) {
      if (!updateCheck && this.lastAddress === this.formData.address) {
        this.editMode = false;
        this.blankScreen = false;

        return;
      }

      this.externalPaymentAddresses[this.coinType] = {
        address: this.formData.address,
        // if not updateCheck, add new address, default to false
        enable: updateCheck ? this.checked : false,
      }
      const data = { externalPaymentAddresses: this.externalPaymentAddresses };
      this.settings.set(data);

      const save = this.settings.save(data, {
        attrs: data,
        type: 'PUT',
      });

      if (save) {
        const msg = {
          msg: app.polyglot.t('wallet.external.statusAddingAddress',
            { coin: `<em>${this.coinName}</em>` }),
          type: 'message',
        };

        const statusMessage = app.statusBar.pushMessage({
          ...msg,
          duration: 99999999999999,
        });

        save.done(() => {
          this.lastAddress = this.externalPaymentAddresses[this.coinType].address;
          this.editMode = false;
          this.blankScreen = false;

          let msgTag = 'wallet.external.statusAddAddressComplete';
          if (updateCheck) {
            msgTag = this.checked ? 'wallet.external.statusAddressEnabled' : 'wallet.external.statusAddressDisabled';
          }
          statusMessage.update({
            msg: app.polyglot.t(msgTag, { coin: `<em>${this.coinName}</em>` }),
            type: 'confirmed',
          });
        }).fail((...args) => {
          let titleTag = 'wallet.external.addAddressErrorAlertTitle';
          let failedMsgTag = 'wallet.external.statusAddAddressFailed';
          if (updateCheck) {
            titleTag = this.checked ? 'wallet.external.enableAddressErrorAlertTitle' : 'wallet.external.disableAddressErrorAlertTitle';
            failedMsgTag = this.checked ? 'wallet.external.statusEnableAddressFailed' : 'wallet.external.statusDisableAddressFailed';
          }

          if (!updateCheck) {
            // restore original externalPaymentAddresses
            this.externalPaymentAddresses[this.coinType].address = this.lastAddress;
            this.settings.set({ externalPaymentAddresses: this.externalPaymentAddresses });
          } else {
            this.checked = !this.checked;
          }
          
          const errMsg = args[0] && args[0].responseJSON && args[0].responseJSON.reason || '';

          openSimpleMessage(
            app.polyglot.t(titleTag, { coin: `<em>${this.coinName}</em>` }),
            errMsg
          );

          statusMessage.update({
            msg: app.polyglot.t(failedMsgTag, { coin: `<em>${this.coinName}</em>` }),
            type: 'warning',
          });
        }).always(() => {
          setTimeout(() => statusMessage.remove(), 3000);
        });
      }
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
  .copy-edit-btn {
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
