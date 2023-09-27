<template>
  <div class="transaction" @click="onDocumentClick">
    <div :class="`flex gutterHSm clrT ${ob.status === 'CONFIRMED' ? 'confirmedTransaction' : ''}`">
      <div :class="`statusIconCol ${statusInfo.statusIconClasses}`"><span :class="statusInfo.statusIcon"></span></div>
      <div class="flexExpand tx5">
        <div class="rowTn flex">{{ infoLine }}</div>
        <div>
          <div class="flexInline gutterH margR clrT2 tx6 floL">
            <div class="flexNoShrink">
              {{
                ob.polyT('wallet.transactions.transaction.timeAgoAndConfirmCount', {
                  timeAgo: ob.timeAgo,
                  confirmationsCount: ob.confirmations,
                  smart_count: ob.confirmations,
                })
              }}
            </div>
            <div class="flexNoShrink" style="max-width: 80px">
              <div class="noOverflow"><a class="clrT2 txU " @click="onClickTxidLink(ob.txid)">{{ ob.txid }}</a></div>
            </div>
          </div>
          <div class="clrT2 tx6" style="vertical-align: text-top;">{{ memoInfo }}</div>
        </div>
      </div>
      <div class="col">
        <div class="flexHRight">
          <div class="btnStrip">
            <a class="btn clrP clrBr clrSh2" :href="ob.walletCur.getBlockChainTxUrl(ob.txid, ob.isTestnet)">{{
              ob.polyT('wallet.transactions.transaction.viewDetailsBtn') }}</a>
            <div v-if="ob.allowFeeBump">
              <ProcessingButton
                :className="`btn clrP clrBr clrSh2 js-retryPmt ${ob.retryInProgress ? 'processing' : ''}`"
                :disabled="ob.retryConfirmOn"
                :btnText="ob.polyT('wallet.transactions.transaction.retryTransactionBtn')"
                @click.stop="onClickRetryPmt" />
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="js-retryPmtConfirmed confirmBox retryConfirm arrowBoxTop clrBr clrP clrT clrSh1" v-show="!!ob.retryConfirmOn">
      <div class="tx3 txB rowSm">{{ ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.title') }}</div>
      <SpinnerSVG v-if="ob.fetchingEstimatedFee" className="txCtr spinnerMd" />

      <div v-else-if="ob.fetchFeeFailed">
        <p class="clrT2 bodyText">{{ ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.fetchError', { err: ob.fetchFeeError || ''}) }}</p>
        <a class=" clrTEm" @click.stop="onClickRetryFeeFetch">{{ ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.btnRetry') }}</a>
      </div>

      <div v-else-if="typeof ob.estimatedFee === 'number'">
        <div v-if="!insufficientFunds">
          <p class="clrT2 bodyText">{{ ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.body', {
            currencyPairing: estimatedFeeCombo,
            asterisk: '<span>*</span>',
          }) }}</p>
        </div>
        <div v-else>
          <p class="clrT2 bodyText">{{
            ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.insufficientFundsBody', {
              currencyPairing: estimatedFeeCombo,
              asterisk: '<span>*</span>',
            }) }}</p>
        </div>
        <p class="clrT2 tx6">{{ ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.subText', {
          asterisk: '<span>*</span>',
        }) }}</p>
      </div>
      <hr class="clrBr row" />
      <div class="flexHRight flexVCent gutterHLg buttonBar">
        <a class="" @click="onClickRetryConfirmCancel">{{ ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.btnCancel') }}</a>
        <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click="onClickRetryConfirmed" :disabled="ob.fetchingEstimatedFee || insufficientFunds">{{ ob.polyT('wallet.transactions.transaction.retryPaymentConfirmBox.btnConfirmSend') }}</a>
      </div>
    </div>
    <div v-if="ob.copiedIndicatorOn">
      <div class="copiedIndicator clrT tx6">Copied to clipboard</div>
    </div>

  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import $ from 'jquery';
import moment from 'moment';
import { ipc } from '../../../../utils/ipcRenderer.js';
import { setTimeagoInterval } from '../../../../../backbone/utils/index.js';
import { getFees } from '../../../../../backbone/utils/fees.js';
import {
  getCurrencyByCode as getWalletCurByCode,
} from '../../../../../backbone/data/walletCurrencies.js';
import app from '../../../../../backbone/app.js';
import { openSimpleMessage } from '../../../../../backbone/views/modals/SimpleMessage';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      const walletBalance = app.walletBalances && app.walletBalances[this.options.coinType];
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        userCurrency: app.settings.get('localCurrency'),
        timeAgo: this.renderedTimeAgo,
        isTestnet: !!app.serverConfig.testnet,
        walletBalance: (walletBalance && walletBalance.toJSON()) || null,
        walletCur: this.walletCur,
        ...this._state,
      };
    },
    renderedTimeAgo () {
      return moment(this.model.get('timestamp')).fromNow();
    },
    statusInfo () {
      let statusIcon = 'ion-ios-checkmark-empty';
      let statusIconClasses = 'clrTEmph1';
      let ob = this.ob;

      if (ob.status === 'DEAD') {
        statusIcon = 'ion-ios-close-empty';
        statusIconClasses = 'tx1 clrTErr';
      } else if (ob.status === 'STUCK') {
        statusIcon = 'ion-alert-circled';
        statusIconClasses = 'tx3 clrTEmph1';
      } else if (ob.status === 'PENDING' || ob.status === 'UNCONFIRMED') {
        statusIcon = 'ion-android-time';
        statusIconClasses = 'tx3';
      }
      return { statusIcon, statusIconClasses };
    },
    infoLine () {
      let ob = this.ob;

      let priceFrag = ob.currencyMod.pairedCurrency(
        Math.abs(ob.value),
        ob.walletCur.code,
        ob.userCurrency
      );

      priceFrag = ob.value < 0 ? `-${priceFrag}` : `+${priceFrag}`;

      let infoLine = '';

      if (ob.value > 0) {
        infoLine = ob.polyT('wallet.transactions.transaction.incomingText', {
          currencyPairing: `<span class="txB flexNoShrink margRTxt clrTEm">${priceFrag}</span>`,
        });
      } else {
        const currencyPairing = `<span class="txB flexNoShrink margRTxt clrTEm">${priceFrag}</span>`;

        if (ob.address) {
          infoLine = ob.polyT('wallet.transactions.transaction.outgoingText', {
            currencyPairing,
            address: `<span class="toAddress margLTxt noOverflow clrTEmph1">${ob.address}</span>`,
          });
        } else {
          infoLine = currencyPairing;
        }

        return infoLine;
      }
    },
    memoInfo () {
      let ob = this.ob;

      return ob.translatedMemo || ob.memo.length > 300 ? `${ob.memo.slice(0, 300)}…` : ob.memo;
    },
    insufficientFunds () {
      let ob = this.ob;

      return ob.walletBalance && ob.walletBalance.confirmed < ob.estimatedFee;
    },
    estimatedFeeCombo () {
      let ob = this.ob;

      return ob.currencyMod.pairedCurrency(ob.estimatedFee, ob.walletCur.code, ob.userCurrency);
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        ...options,
        initialState: {
          retryConfirmOn: false,
          retryInProgress: false,
          copiedIndicatorOn: false,
          fetchingEstimatedFee: false,
          fetchFeeError: '',
          fetchFeeFailed: false,
          ...options.initialState,
        },
      };

      this.baseInit(opts);

      if (!this.model) {
        throw new Error('Please provide a Transaction model.');
      }

      if (typeof opts.coinType !== 'string') {
        throw new Error('Please provide a coinType as a string.');
      }

      this.walletCur = getWalletCurByCode(opts.coinType);

      if (opts.bumpFeeXhr) {
        this.onPostBumpFee(opts.bumpFeeXhr, {
          // These both will already happen since the fee bump was initiated from this
          // view. Let's prevent them from happening a duplicate time.
          triggerBumpFeeAttempt: false,
          showErrorOnFail: false,
        });
      }
    },

    onDocumentClick (e) {
      let retryPmtConfirmedBox = $('.js-retryPmtConfirmed');
      if (this.getState().retryConfirmOn &&
        !($.contains(retryPmtConfirmedBox[0], e.target) ||
          e.target === retryPmtConfirmedBox[0])) {
        this.setState({
          retryConfirmOn: false,
        });
      }
    },

    onClickRetryFeeFetch (e) {
      this.fetchFees();
    },

    onPostBumpFee (xhr, options = {}) {
      const opts = {
        triggerBumpFeeAttempt: true,
        showErrorOnFail: true,
        ...options,
      };

      if (
        !xhr ||
        typeof xhr.done !== 'function' &&
        typeof xhr.fail !== 'function' &&
        typeof xhr.always !== 'function'
      ) {
        throw new Error('Please provide a jQuery xhr');
      }

      this.setState({
        retryInProgress: true,
        retryConfirmOn: false,
      });

      xhr.always(() => {
        this.setState({
          retryInProgress: false,
        });
      }).fail((failXhr) => {
        if (opts.showErrorOnFail) {
          if (failXhr.statusText === 'abort') return;
          const failReason = (failXhr.responseJSON && failXhr.responseJSON.reason) || '';
          openSimpleMessage(
            app.polyglot.t('wallet.transactions.transaction.retryFailDialogTitle'),
            failReason,
          );
        }
      })
        .done((data) => {
          this.$emit('bumpFeeSuccess', {
            md: this.model,
            data,
          });
          this.model.set('feeBumped', true);
        });

      if (opts.triggerBumpFeeAttempt) {
        this.$emit('bumpFeeAttempt', {
          md: this.model,
          xhr,
        });
      }
    },

    onClickRetryConfirmed () {
      const post = $.post(app.getServerUrl(`wallet/bumpfee/${this.model.id}`));
      this.onPostBumpFee(post);
    },

    onClickRetryPmt (e) {
      this.setState({
        retryConfirmOn: true,
      });

      this.fetchFees();
    },

    onClickRetryConfirmCancel () {
      this.closeRetryConfirmBox();
    },

    onClickTxidLink (txid) {
      this.setState({
        copiedIndicatorOn: true,
      });

      ipc.send('controller.system.writeToClipboard', txid);
      clearTimeout(this.copiedIndicatorTimeout);

      this.copiedIndicatorTimeout = setTimeout(() => {
        this.setState({
          copiedIndicatorOn: false,
        });
      }, 1000);
    },

    fetchFees () {
      this.setState({
        retryConfirmOn: true,
        fetchingEstimatedFee: true,
        fetchFeeError: '',
        fetchFeeFailed: false,
      });

      getFees(this.options.coinType).done((fees) => {
        if (this.isRemoved()) return;
        this.setState({
          fetchingEstimatedFee: false,
          // server doubles the fee when bumping
          estimatedFee: this.walletCur.feeBumpTransactionSize * fees.priority * 2,
        });
      }).fail((reason) => {
        if (this.isRemoved()) return;
        this.setState({
          fetchingEstimatedFee: false,
          fetchFeeFailed: true,
          fetchFeeError: reason || '',
        });
      });
    },

    closeRetryConfirmBox () {
      this.setState({
        retryConfirmOn: false,
        fetchingEstimatedFee: false,
      });
    },

    remove () {
      clearTimeout(this.copiedIndicatorTimeout);
    },
  }
}
</script>
<style lang="scss" scoped></style>
