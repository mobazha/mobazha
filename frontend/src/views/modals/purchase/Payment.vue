<template>
  <div class="pending">
    <!--
  This view is used by both the Purchase and Order detail flows. When making changes, please ensure they play nice with both areas.
-->

    <div :class="`flexRow gutterHLg ${ob.externallyFundable ? `pad rowMd` : ''}`">
      <template v-if="ob.externallyFundable">
        <div class="flexNoShrink">
          <a class="QR js-purchaseQRCode"><img class="js-qrCodeImg" :src="ob.qrDataUri"></a>
        </div>
      </template>
      <div class="flexExpand">
        <div class="flexVCent">
          <div class="flexExpand">
            <div class="rowSm clickable " @click="copyAmount">
              <span class="h1">{{ ob.amountDueLine }}</span>
              <template v-if="ob.externallyFundable">
                <button :class="`btnTxtOnly txUnb flipBtn js-copyAmount ${copyAmountActive ? 'active' : ''}`">
                  <span class="clrTEm unFlipped">{{ ob.polyT('purchase.pendingSection.copy') }}</span>
                  <span class="flipped">{{ ob.polyT('purchase.pendingSection.copied') }}</span>
                </button>
              </template>
            </div>
            <template v-if="ob.externallyFundable">
              <div class="tx5 rowMd clickable " @click="copyAddress(ob.paymentAddress)">
                <span :class="ob.paymentAddress.length > 34 ? 'toolTipNoWrap toolTipTop' : ''"
                  :data-tip="ob.paymentAddress.length > 34 ? ob.paymentAddress : ''">
                  {{ ob.polyT('purchase.pendingSection.to', { address: pAddress }) }}
                </span>
                <button :class="`btnTxtOnly txUnb flipBtn js-copyAddress ${copyAddressActive ? 'active' : ''}`">
                  <span class="clrTEm unFlipped">{{ ob.polyT('purchase.pendingSection.copy') }}</span>
                  <span class="flipped">{{ ob.polyT('purchase.pendingSection.copied') }}</span>
                </button>
              </div>
            </template>
            <div :class="`flexRow gutterH ${ob.externallyFundable ? 'rowLg' : 'rowMd'}`">
              <div class="col6">
                <ProcessingButton
                  :className="`btn btnThin width100 clrP clrBr clrSh2 js-payFromWallet ${isPaying ? 'processing' : ''}`"
                  textClassName="flexCent"
                  :btnText='`<i class="icon"><WalletIcon /></i>${ob.polyT("purchase.pendingSection.payFromWallet")}`'
                  @click.stop="clickPayFromWallet" />
                <div ref="confirmWalletContainer" class="js-confirmWalletContainer">
                  <SpendConfirmBox
                    ref="spendConfirmBox"
                    :options="{
                      metricsOrigin,
                      initialState: {
                        btnSendText: ob.polyT('purchase.pendingSection.btnConfirmedPay'),
                        coinType: paymentCoin,
                      },
                    }"
                    @clickSend="walletConfirm" />
                </div>
              </div>
            </div>
            <template v-if="ob.externallyFundable">
              <div class="txBase clrT2">
                <template v-if="['BTC', 'TBTC'].includes(paymentCoin)">
                  <p>
                    {{ ob.polyT('purchase.pendingSection.walletNote') }} <button class="btnAsLink "
                      @click="clickFundWallet">{{ ob.polyT('purchase.pendingSection.walletLink') }}</button>
                  </p>
                </template>
                <p v-html="ob.polyT('purchase.pendingSection.feeNote')"></p>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>
    <div class="tx6 clrT2">
      <template v-if="['BTC', 'TBTC'].includes(paymentCoin)">
        <p v-html='ob.polyT("purchase.pendingSection.note1", {
          link: `<a class="clrTEm" href="https://www.openbazaar.org/bitcoin">${ob.polyT("purchase.pendingSection.note1Link")}</a>`,
        })'></p>
      </template>
      <template v-if="ob.isModerated">
        <p>
          {{ ob.polyT('purchase.pendingSection.note2') }}
          <br>
          {{ ob.polyT('purchase.pendingSection.note3') }}
        </p>
      </template>
      <p>{{ ob.polyT('purchase.pendingSection.note4') }}</p>
    </div>

    <Teleport to="#js-vueModal">
      <Wallet ref="walletModal" v-show="showWallet" @close="closeWallet" />
    </Teleport>
  </div>
</template>

<script>
/*
This view is also used by the Order Detail overlay. If you make any changes, please
ensure they are compatible with both the Purchase and Order Detail flows.
*/

import bigNumber from 'bignumber.js';
import qr from 'qr-encode';
import { ipc } from '../../../utils/ipcRenderer';
import app from '../../../../backbone/app';
import {
  formatCurrency,
  integerToDecimal,
  getCoinDivisibility,
} from '../../../../backbone/utils/currency';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../backbone/data/walletCurrencies';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import { orderSpend } from '../../../../backbone/models/wallet/Spend';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import {
  startPrefixedAjaxEvent,
  endPrefixedAjaxEvent,
  recordPrefixedEvent,
} from '../../../../backbone/utils/metrics';

import Wallet from '@/views/modals/wallet/Wallet.vue';
import SpendConfirmBox from '../wallet/SpendConfirmBox.vue';

export default {
  components: {
    SpendConfirmBox,
    Wallet,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      app,

      copyAmountActive: false,
      copyAddressActive: false,
      isPaying: false,

      showWallet: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  unmounted() {
    if (this.hideCopyAmountTimer) {
      clearTimeout(this.hideCopyAmountTimer);
    }
  },
  computed: {
    ob () {
      return {
          ...this.templateHelpers,
          amountDueLine: this.amountDueLine,
          paymentAddress: this.paymentAddress,
          qrDataUri: this.qrDataUri,
          isModerated: this.isModerated,
          externallyFundable: this.paymentCoinData.externallyFundableOrders,
      };
    },

    pAddress() {
      const ob = this.ob;

      return ob.paymentAddress.length > 34 ? `${ob.paymentAddress.slice(0, 34)}…` : ob.paymentAddress;
    },

    amountDueLine () {
      let amountBTC = '';

      try {
        amountBTC = formatCurrency(this.balanceRemaining, this.paymentCoin);
      } catch (e) {
        // pass
      }

      return app.polyglot.t('purchase.pendingSection.pay', { amountBTC });
    },
    
    qrDataUri () {
      const address = this.paymentCoinData.qrCodeText(this.paymentAddress);
      const URL = `${address}?amount=${this.balanceRemaining}`;
      return qr(URL, { type: 8, size: 5, level: 'M' });
    },
  },
  methods: {
    loadData (options = {}) {
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

      this.baseInit(options);

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
                  this.isPaying = false;
                  this.$emit('walletPaymentComplete', e.jsonData.notification);
                } else {
                  this.balanceRemaining = this.balanceRemaining.minus(amount);
                }
              }
            }
          }
        });
      }
    },

    clickPayFromWallet () {
      const walletBalance = app.walletBalances.get(this.paymentCoin);
      const insufficientFunds = this.balanceRemaining.gt(walletBalance ? walletBalance.get('confirmed') : 0);

      if (insufficientFunds) {
        this.$refs.spendConfirmBox.setState({
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
        this.$refs.spendConfirmBox.setState({ show: true });
        this.$refs.spendConfirmBox.fetchFeeEstimate(this.balanceRemaining);
      }
    },

    showSpendError (error = '') {
      openSimpleMessage(app.polyglot.t('purchase.errors.paymentFailed'), error);
    },

    walletConfirm () {
      this.isPaying = true;
      this.$refs.spendConfirmBox.setState({ show: false });
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
            const err = jqXhr.responseJSON && jqXhr.responseJSON.reason || jqXhr.responseText;
            this.showSpendError(err);
            endPrefixedAjaxEvent('SpendFromWallet', this.metricsOrigin, {
              currency,
              errors: err || 'unknown error',
            });
            if (this.isRemoved()) return;
            this.isPaying = false;
          });
      } catch (e) {
        // This is almost certainly a dev error if this happens, but it prevents the purchase and
        // is confusing and at least to make debugging easier, we'll display an error modal.
        this.showSpendError(e.message || '');
        this.isPaying = false;
      }
    },

    copyAmount () {
      ipc.send('controller.system.writeToClipboard', String(this.balanceRemaining));

      this.copyAmountActive = true;
      if (this.hideCopyAmountTimer) {
        clearTimeout(this.hideCopyAmountTimer);
      }
      this.hideCopyAmountTimer = setTimeout(() => this.copyAmountActive = false, 3000);
    },

    copyAddress () {
      ipc.send('controller.system.writeToClipboard', String(this.paymentAddress));

      this.copyAddressActive = true;
      if (this.hideCopyAddressTimer) {
        clearTimeout(this.hideCopyAddressTimer);
      }
      this.hideCopyAddressTimer = setTimeout(() => this.copyAddressActive = false, 3000);
    },

    clickFundWallet () {
      this.showWallet = true;

      this.$refs.walletModal.tabActive = 'receive';
    },

    closeWallet() {
      this.showWallet = false;
    }
  }
}
</script>
<style lang="scss" scoped></style>
