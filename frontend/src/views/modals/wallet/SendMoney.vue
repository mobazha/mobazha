<template>
  <div class="sendMoney">
    <form class="padMdKids padStack">
      <div class="flexRow gutterHLg">
        <div class="col2">
          <div class="flexVCent flexHRight">
            <label for="walletSendTo" class="required">{{ ob.polyT('wallet.sendMoney.toLabel') }}</label>
          </div>
        </div>
        <div class="col9">
          <FormError v-if="ob.errors.address" :errors="ob.errors.address" />
          <input type="text" class="clrBr clrSh2" :disabled="ob.saveInProgress" name="address" id="walletSendTo"
            ref="walletSendTo"
            v-model="ob.address"
            :placeholder="ob.polyT('wallet.sendMoney.toPlaceholder', { cur: ob.polyT(`cryptoCurrencies.${ob.coinType}`, { _: ob.coinType }) })">
        </div>
      </div>
      <div class="flexRow gutterHLg">
        <div class="col2">
          <div class="flexVCent flexHRight">
            <label for="walletSendAmount" class="required">{{ ob.polyT('wallet.sendMoney.amountLabel') }}</label>
          </div>
        </div>
        <div class="col9">
          <FormError v-if="ob.errors.amount" :errors="ob.errors.amount" />
          <FormError v-if="ob.errors.currency" :errors="ob.errors.currency" />
          <div class="inputSelect" :disabled="ob.saveInProgress">
            <input type="text" class="clrBr clrP clrSh2" name="amount" id="walletSendAmount"
              :value="ob.number.toStandardNotation(ob.amount)" placeholder="0.00" data-var-type="bignumber">
            <select id="walletSendCurrency" name="currency" class="clrBr clrP nestInputRight">
              <option v-for="(currency, j) in ob.currencies" :key="j" :value="currency.code" :selected="currency.code === ob.currencyCode">{{ currency.code }}</option>
            </select>
          </div>
        </div>
      </div>
      <div class="flexRow gutterHLg">
        <div class="col2">
          <div class="flexVCent flexHRight">
            <label for="walletSendNote">{{ ob.polyT('wallet.sendMoney.noteLabel') }}</label>
          </div>
        </div>
        <div class="col9">
          <FormError v-if="ob.errors.memo" :errors="ob.errors.memo" />
          <input type="text" class="clrBr clrSh2" :disabled="ob.saveInProgress" name="memo" id="walletSendNote"
            v-model="ob.memo" :placeholder="ob.polyT('wallet.sendMoney.notePlaceholder')">
        </div>
      </div>
      <div class="flexVCent">
        <div class="flexExpand">
          <div class="flexHRight flexVCent gutterH col11">
            <a class=" flexNoShrink" @click="onClickClear">{{ ob.polyT('wallet.sendMoney.clear') }}</a>
            <div class="posR">
              <ProcessingButton
                :className="`js-btnSend btn clrBAttGrad clrBrDec1 clrTOnEmph ${ob.saveInProgress ? 'processing' : ''}`"
                :btnText="ob.polyT('wallet.sendMoney.sendBtn')" @click.stop="onClickSend" />
              <div class="js-sendConfirmContainer"></div>
              <SpendConfirmBox ref="spendConfirmBox" :options="{ metricsOrigin: 'Wallet',}" @clickSend="onClickConfirmSend" />
            </div>
          </div>
        </div>
      </div>
    </form>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import { getCurrenciesSortedByCode } from '../../../../backbone/data/currencies';
import { endAjaxEvent, recordEvent, startAjaxEvent } from '../../../../backbone/utils/metrics';
import { convertCurrency, getExchangeRate } from '../../../../backbone/utils/currency';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import Spend, { spend } from '../../../../backbone/models/wallet/Spend';
import SpendConfirmBox from './SpendConfirmBox.vue';


export default {
  components: {
    SpendConfirmBox,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      saveInProgress: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    $('#walletSendCurrency').select2();
  },
  computed: {
    defaultCur () {
      return typeof getExchangeRate(app.settings.get('localCurrency')) === 'number' ?
        app.settings.get('localCurrency') : this.coinType;
    },
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        errors: this.model.validationError || {},
        currencyCode: this.model.get('currency') || this.defaultCur,
        currencies: this.currencies || getCurrenciesSortedByCode(),
        saveInProgress: this.saveInProgress,
        coinType: this.coinType,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (typeof options.coinType !== 'string' || !options.coinType) {
        throw new Error('Please provide the coinType as a string.');
      }

      this.baseInit(options);
      this.coinType = options.coinType;
      this.model = new Spend({ wallet: options.coinType });
    },

    onClickConfirmSend () {
      this.$refs.spendConfirmBox.setState({ show: false });

      // POSTing payment to the server
      this.saveInProgress = true;

      startAjaxEvent('Wallet_SendConfirm');

      spend({
        ...this.model.toJSON(),
        coinType: this.coinType,
        feeLevel: app.localSettings.get('defaultTransactionFee'),
      }).fail(jqXhr => {
        let reason = jqXhr.responseText || '';

        if (reason === 'ERROR_INVALID_ADDRESS') {
          reason = app.polyglot.t('wallet.sendMoney.errorInvalidAddress');
        }

        openSimpleMessage(app.polyglot.t('wallet.sendMoney.sendPaymentFailDialogTitle'), reason);
        endAjaxEvent('Wallet_SendConfirm', {
          errors: reason,
        });
      }).always(() => {
        this.saveInProgress = false;
      })
        .done(() => {
          endAjaxEvent('Wallet_SendConfirm');
          this.clearForm();
        });
    },

    onClickSend () {
      const formData = this.getFormData(this.getFormFields());
      this.model.set(formData);
      this.model.set({}, { validate: true });

      if (!this.model.validationError) {
        recordEvent('Wallet_Send', { coin: this.coinType });
        this.$refs.spendConfirmBox.setState({ show: true });
        this.fetchFeeEstimate();
      }

      const $firstErr = $('.errorList:first');
      if ($firstErr.length) $firstErr[0].scrollIntoViewIfNeeded();
    },

    onClickClear () {
      this.clearForm();
    },

    focusAddress () {
      if (!this.saveInProgress) this.$refs.walletSendTo.focus();
    },

    setFormData (data = {}, options = {}) {
      const opts = {
        focusAddressInput: true,
        render: true,
        ...options,
      };

      this.clearForm();
      this.model.set(data);

      setTimeout(() => {
        if (opts.focusAddressInput) this.focusAddress();
      });
    },

    clearModel () {
      // this.model.clear();

      // for some reason model.clear is not working, so we'll go
      // with a manual approach
      this.model.unset('address');
      this.model.unset('amount');
      this.model.unset('memo');
      this.model.unset('currency');
      this.model.set(this.model.defaults || {});
      this.model.validationError = null;
    },

    clearForm () {
      this.clearModel();
    },

    fetchFeeEstimate () {
      const amount = convertCurrency(this.model.get('amount'), this.model.get('currency'),
        this.coinType);
      this.$refs.spendConfirmBox.fetchFeeEstimate(amount, this.coinType);
    },

    getFormFields () {
      return $(`select[name], input[name], 
        textarea[name]:not([class*="trumbowyg"]), 
        div[contenteditable][name]`);
    },
  }
}
</script>
<style lang="scss" scoped></style>
