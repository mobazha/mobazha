<template>
  <div class="pending">
    <!--
  This view is used by both the Purchase and Order detail flows. When making changes, please ensure they play nice with both areas.
-->

    <div :class="`flexRow gutterHLg ${ob.externallyFundable ? `pad rowMd` : ''}`">
      <div v-if="ob.externallyFundable">
        <div class="flexNoShrink">
          <a class="QR js-purchaseQRCode"><img class="js-qrCodeImg" :src="ob.qrDataUri"></a>
        </div>
      </div>
      <div class="flexExpand">
        <div class="flexVCent">
          <div class="flexExpand">
            <div class="rowSm clickable " @click="copyAmount">
              <span class="h1 js-amountDueLine">{{ ob.amountDueLine }}</span>
              <div v-if="ob.externallyFundable">
                <button class="btnTxtOnly txUnb flipBtn js-copyAmount">
                  <span class="clrTEm unFlipped">{{ ob.polyT('purchase.pendingSection.copy') }}</span>
                  <span class="flipped">{{ ob.polyT('purchase.pendingSection.copied') }}</span>
                </button>
              </div>
            </div>
            <div v-if="ob.externallyFundable">
              <div class="tx5 rowMd clickable " @click="copyAddress">
                <span :class="ob.paymentAddress.length > 34 ? 'toolTipNoWrap toolTipTop' : ''" :data-tip="ob.paymentAddress.length > 34 ? ob.paymentAddress : ''">
                  {{ ob.polyT('purchase.pendingSection.to', { address: pAddress }) }}
                </span>
                <button class="btnTxtOnly txUnb flipBtn js-copyAddress">
                  <span class="clrTEm unFlipped">{{ ob.polyT('purchase.pendingSection.copy') }}</span>
                  <span class="flipped">{{ ob.polyT('purchase.pendingSection.copied') }}</span>
                </button>
              </div>
            </div>
            <div :class="`flexRow gutterH ${ob.externallyFundable ? 'rowLg' : 'rowMd'}`">
              <div class="col6">
                <%= ob.processingButton({
            className: 'btn btnThin width100 clrP clrBr clrSh2 js-payFromWallet',
            textClassName: 'flexCent',
            btnText: `<i class="icon">${ob.walletIconTmpl()}</i>${ob.polyT('purchase.pendingSection.payFromWallet')}`,
            }) %>
                <div class="js-confirmWalletContainer"></div>
              </div>
            </div>
            <div v-if="ob.externallyFundable">
              <div class="txBase clrT2">
                <div v-if="['BTC', 'TBTC'].includes(ob.paymentCoin)">
                  <p>
                    {{ ob.polyT('purchase.pendingSection.walletNote') }} <button class="btnAsLink " @click="clickFundWallet">{{ ob.polyT('purchase.pendingSection.walletLink') }}</button>
                  </p>
                </div>
                <p>
                  {{ ob.polyT('purchase.pendingSection.feeNote') }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="tx6 clrT2">
      <div v-if="['BTC', 'TBTC'].includes(ob.paymentCoin)">
        <p>{{ ob.polyT('purchase.pendingSection.note1', {
      link: `<a class="clrTEm" href="https://www.openbazaar.org/bitcoin">${ob.polyT('purchase.pendingSection.note1Link')}</a>`,
     }) }}</p>
      </div>
      <div v-if="ob.isModerated">
        <p>
          {{ ob.polyT('purchase.pendingSection.note2') }}
          <br>
          {{ ob.polyT('purchase.pendingSection.note3') }}
        </p>
      </div>
      <p>{{ ob.polyT('purchase.pendingSection.note4') }}</p>
    </div>

  </div>
</template>

<script setup>
/*
This view is also used by the Order Detail overlay. If you make any changes, please
ensure they are compatible with both the Purchase and Order Detail flows.
*/

import bigNumber from 'bignumber.js';
import qr from 'qr-encode';
import { ipc } from '../../../utils/ipcRenderer.js';
import app from '../../../../backbone/app.js';
import loadTemplate from '../../../../backbone/utils/loadTemplate.js';
import {
  formatCurrency,
  integerToDecimal,
  getCoinDivisibility,
} from '../../../../backbone/utils/currency.js';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../backbone/data/walletCurrencies.js';
import { getSocket } from '../../../../backbone/utils/serverConnect.js';
import BaseVw from '../../baseVw';
import SpendConfirmBox from '../wallet/SpendConfirmBox';
import { orderSpend } from '../../../../backbone/models/wallet/Spend.js';
import { openSimpleMessage } from '../SimpleMessage';
import { launchWallet } from '../../../../backbone/utils/modalManager.js';
import {
  startPrefixedAjaxEvent,
  endPrefixedAjaxEvent,
  recordPrefixedEvent,
} from '../../../../backbone/utils/metrics.js';


const props = defineProps({
  phase: String,
})

const pAddress = ob.paymentAddress.length > 34 ? `${ob.paymentAddress.slice(0, 34)}…` : ob.paymentAddress;

loadData(props);

render();

function loadData (options = {}) {
  if (!(options.balanceRemaining instanceof bigNumber)) {
    throw new Error('Please provide the balance remaining (in the server\'s'
      + ' currency) as a bigNumber instance.');
  }

  if (!options.paymentAddress) {
    throw new Error('Please provide the payment address.');
  }

  if (!options.orderID) {
    throw new Error('Please provide an orderID.');
  }

  if (typeof options.isModerated !== 'boolean') {
    throw new Error('Please provide a boolean indicating whether the order is moderated.');
  }

  if (!options.metricsOrigin) {
    throw new Error('Please provide an origin for the metrics reporting');
  }

  let paymentCoinData;

  try {
    paymentCoinData = getWalletCurByCode(options.paymentCoin);
  } catch (e) {
    // pass
  }

  if (!paymentCoinData) {
    throw new Error(`Unable to obtain wallet currency data for "${options.paymentCoin}"`);
  }

  super(options);

  this.options = options;
  this._balanceRemaining = options.balanceRemaining;
  this.paymentAddress = options.paymentAddress;
  this.orderID = options.orderID;
  this.isModerated = options.isModerated;
  this.metricsOrigin = options.metricsOrigin;
  this.paymentCoin = options.paymentCoin;
  this.paymentCoinData = paymentCoinData;

  const serverSocket = getSocket();
  if (serverSocket) {
    this.listenTo(serverSocket, 'message', (e) => {
      // listen for a payment socket message, to react to payments from all sources
      if (e.jsonData.notification && e.jsonData.notification.type === 'orderPaymentReceived') {
        if (e.jsonData.notification.orderID === this.orderID) {
          let amount;

          try {
            const coinDiv = getCoinDivisibility(this.paymentCoin);

            amount = integerToDecimal(
              e.jsonData.notification.fundingTotal,
              coinDiv,
              { returnNaNOnError: false },
            );
          } catch (err) {
            console.error('Unable to convert the payment notification amount '
              + `from base units: ${err}`);
          }

          if (amount && amount.isNaN && !amount.isNaN()) {
            if (amount.gte(this.balanceRemaining)) {
              this.getCachedEl('.js-payFromWallet').removeClass('processing');
              this.trigger('walletPaymentComplete', e.jsonData.notification);
            } else {
              this.balanceRemaining = this.balanceRemaining.minus(amount);
            }
          }
        }
      }
    });
  }
}

  set balanceRemaining(amount) {
  if (!(amount instanceof bigNumber)) {
    throw new Error('Please provide an amount as a BigNumber instance.');
  }

  if (!amount.eq(this._balanceRemaining)) {
    this._balanceRemaining = amount;
    this.getCachedEl('.js-amountDueLine').html(this.amountDueLine);
    this.getCachedEl('.js-qrCodeImg').attr('src', this.qrDataUri);
  }
}

  get balanceRemaining() {
  return this._balanceRemaining;
}

function events () {
  return {
    'click .js-payFromWallet': 'clickPayFromWallet',
  };
}

function clickPayFromWallet (e) {
  const walletBalance = app.walletBalances.get(this.paymentCoin);
  const insufficientFunds = this.balanceRemaining.gt(walletBalance ? walletBalance.get('confirmed') : 0);

  if (insufficientFunds) {
    this.spendConfirmBox.setState({
      show: true,
      fetchFailed: true,
      fetchError: 'ERROR_INSUFFICIENT_FUNDS',
    });
    recordPrefixedEvent('PayFromWallet', this.metricsOrigin, {
      currency: this.paymentCoin,
      sufficientFunds: false,
    });
  } else {
    recordPrefixedEvent('PayFromWallet', this.metricsOrigin, {
      currency: this.paymentCoin,
      sufficientFunds: true,
    });
    this.spendConfirmBox.setState({ show: true });
    this.spendConfirmBox.fetchFeeEstimate(this.balanceRemaining);
  }

  e.stopPropagation();
}

function showSpendError (error = '') {
  openSimpleMessage(app.polyglot.t('purchase.errors.paymentFailed'), error);
}

function walletConfirm () {
  this.getCachedEl('.js-payFromWallet').addClass('processing');
  this.spendConfirmBox.setState({ show: false });
  const currency = this.paymentCoin;

  startPrefixedAjaxEvent('SpendFromWallet', this.metricsOrigin);

  try {
    orderSpend({
      orderID: this.orderID,
      address: this.paymentAddress,
      amount: this.balanceRemaining,
      coinType: currency,
      currency,
      wallet: currency,
    })
      .done(() => {
        endPrefixedAjaxEvent('SpendFromWallet', this.metricsOrigin, { currency });
      })
      .fail((jqXhr) => {
        const err = jqXhr.responseText || '';
        showSpendError(err);
        endPrefixedAjaxEvent('SpendFromWallet', this.metricsOrigin, {
          currency,
          errors: err || 'unknown error',
        });
        if (this.isRemoved()) return;
        this.getCachedEl('.js-payFromWallet').removeClass('processing');
      });
  } catch (e) {
    // This is almost certainly a dev error if this happens, but it prevents the purchase and
    // is confusing and at least to make debugging easier, we'll display an error modal.
    showSpendError(e.message || '');
    this.getCachedEl('.js-payFromWallet').removeClass('processing');
  }
}

function copyAmount () {
  ipc.send('controller.system.writeToClipboard', String(this.balanceRemaining));

  this.getCachedEl('.js-copyAmount').addClass('active');
  if (this.hideCopyAmountTimer) {
    clearTimeout(this.hideCopyAmountTimer);
  }
  this.hideCopyAmountTimer = setTimeout(() => this.getCachedEl('.js-copyAmount').removeClass('active'), 3000);
}

function copyAddress () {
  ipc.send('controller.system.writeToClipboard', String(this.paymentAddress));

  this.getCachedEl('.js-copyAddress').addClass('active');
  if (this.hideCopyAddressTimer) {
    clearTimeout(this.hideCopyAddressTimer);
  }
  this.hideCopyAddressTimer = setTimeout(() => this.getCachedEl('.js-copyAddress').removeClass('active'), 3000);
}

function clickFundWallet () {
  launchWallet().sendModeOn = false;
}

  get amountDueLine() {
  let amountBTC = '';

  try {
    amountBTC = formatCurrency(this.balanceRemaining, this.paymentCoin);
  } catch (e) {
    // pass
  }

  return app.polyglot.t('purchase.pendingSection.pay', { amountBTC });
}

  get qrDataUri() {
  const address = this.paymentCoinData.qrCodeText(this.paymentAddress);
  const URL = `${address}?amount=${this.balanceRemaining}`;
  return qr(URL, { type: 8, size: 5, level: 'M' });
}

function remove () {
  if (this.hideCopyAmountTimer) {
    clearTimeout(this.hideCopyAmountTimer);
  }
  super.remove();
}

function render () {
  super.render();
  const displayCurrency = app.settings.get('localCurrency');

  loadTemplate('modals/purchase/payment.html', (t) => {
    loadTemplate('walletIcon.svg', (walletIconTmpl) => {
      this.$el.html(t({
        displayCurrency,
        amountDueLine: this.amountDueLine,
        paymentAddress: this.paymentAddress,
        qrDataUri: this.qrDataUri,
        walletIconTmpl,
        isModerated: this.isModerated,
        paymentCoin: this.paymentCoin,
        externallyFundable: this.paymentCoinData.externallyFundableOrders,
      }));
    });

    this.spendConfirmBox = this.createChild(SpendConfirmBox, {
      metricsOrigin: this.metricsOrigin,
      initialState: {
        btnSendText: app.polyglot.t('purchase.pendingSection.btnConfirmedPay'),
        coinType: this.paymentCoin,
      },
    });
    this.listenTo(this.spendConfirmBox, 'clickSend', this.walletConfirm);
    this.getCachedEl('.js-confirmWalletContainer').html(this.spendConfirmBox.render().el);
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
