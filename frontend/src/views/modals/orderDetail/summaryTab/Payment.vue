<template>
  <div class="payment rowLg">

    <h2 class="tx4 margRTn">{{ heading }}</h2>
    <div v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ ob.moment(ob.timestamp).format('lll') }}</span>
    </div>
    <div class="border clrBr padMd">
      <div class="flexVCent gutterH clrT">
        <!-- // if ob.amountShort.gt(0), it is partial payment, otherwise full payment -->
        <div :class="`statusIconCol ${amountShort.gt(0) ? 'clrTErr' : 'clrTEm'}`">
          <div v-if="!ob.isCrypto">
            <span :class="`clrBr ${amountShort.gt(0) ? 'ion-ios-close-empty' : 'ion-ios-checkmark-empty'}`"></span>
          </div>

          <div v-else>
            {{ ob.crypto.cryptoIcon({ code: ob.paymentCoin, className: 'clrBr', }) }}
          </div>
        </div>
        <div class="flexExpand tx5">
          <div class="rowTn txB">{{ infoLine }}</div>
          <div class="flex gutterH">
            <div style="flex-shrink: 0">{{ confirmationsText }}</div>
            <div style="flex-shrink: 0;max-width: 80px">
              <div class="noOverflow">
                <div v-if="ob.blockChainTxUrl">
                  <a class="clrT2 js-txidLink" :href="ob.blockChainTxUrl">{{ ob.txid }}</a>
                </div>

                <div v-else>
                  <span class="clrT2">{{ ob.txid }}</span>
                </div>
              </div>
            </div>
            <div>
              <div :class="`noOverflow ${ob.amountShort.gt(0) ? 'clrTErr' : 'clrTEm'}`">{{ subText }}</div>
            </div>
          </div>
        </div>
        <div v-if="ob.showAcceptRejectButtons || ob.showCancelButton">
          <div class="col">
            <div class="flexVCent gutterHLg">
              <div v-if="ob.showAcceptRejectButtons">
                <div class="flexVCent gutterHLg">
                  <div v-if="ob.rejectInProgress">
                    <span class="posR">
                      <!-- // including invisible cancel link to properly space the spinner -->
                      <a class="txU tx6 invisible">{{ ob.polyT('orderDetail.summaryTab.payment.rejectBtn') }}</a>
                      <SpinnerSVG className="spinnerSm center" />
                    </span>
                  </div>

                  <div v-else>
                    <div class="posR">
                      <a class="txU tx6" :disabled="ob.acceptInProgress" @click="onClickRejectOrder">{{ ob.polyT('orderDetail.summaryTab.payment.rejectBtn') }}</a>
                      <div class=" confirmBox rejectConfirm tx5 arrowBoxTop clrBr clrP clrT" @click="onClickRejectConfirmBox" :hidden="!ob.rejectConfirmOn">
                        <div class="tx3 txB rowSm">{{ ob.polyT('orderDetail.summaryTab.payment.rejectConfirm.title') }}</div>
                        <p>
                          {{
                            ob.polyT('orderDetail.summaryTab.payment.rejectConfirm.body', {
                              cur: ob.polyT(`cryptoCurrencies.${ob.paymentCoin}`, { _: ob.paymentCoin }),
                            })
                          }}
                        </p>
                        <hr class="clrBr row" />
                        <div class="flexHRight flexVCent gutterHLg buttonBar">
                          <a class="" @click="onClickRejectConfirmCancel">{{ ob.polyT('orderDetail.summaryTab.payment.rejectConfirm.btnCancel') }}</a>
                          <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph " @click="onClickRejectConfirmed">{{ ob.polyT('orderDetail.summaryTab.payment.rejectConfirm.btnConfirm') }}</a>
                        </div>
                      </div>
                    </div>
                  </div>
                  <ProcessingButton
                    :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph tx5b js-acceptOrder ${ob.acceptInProgress ? 'processing' : ''}`"
                    :disabled="ob.rejectInProgress" :btnText="ob.polyT('orderDetail.summaryTab.payment.acceptBtn')"
                    @click="onClickAcceptOrder" />
                </div>
              </div>

              <div v-else-if="ob.showCancelButton">
                <div v-if="ob.cancelInProgress">
                  <span class="posR">
                    <!-- // including invisible cancel link to properly space the spinner -->
                    <a class="txU tx6 invisible">{{ ob.polyT('orderDetail.summaryTab.payment.cancelBtn') }}</a>
                    <SpinnerSVG className="spinnerSm center" />
                  </span>
                </div>

                <div v-else>
                  <a class="txU tx6 " @click="onClickCancelOrder">{{ ob.polyT('orderDetail.summaryTab.payment.cancelBtn')
                  }}</a>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import moment from 'moment';
import bigNumber from 'bignumber.js';
import app from '../../../../../backbone/app';
import { abbrNum } from '../../../../../backbone/utils';
import { integerToDecimal } from '../../../../../backbone/utils/currency';


export default {
  mixins: [],
  props: {
    cart: Object,
  },
  data () {
    return {
      info: {
        paymentNumber: 1,
        amountShort: bigNumber(0),
        balanceRemaining: bigNumber(0),
        payee: '',
        userCurrency: app.settings.get('localCurrency') || 'BTC',
        showAcceptRejectButtons: false,
        showCancelButton: false,
        acceptInProgress: false,
        rejectInProgress: false,
        cancelInProgress: false,
        blockChainTxUrl: '',
        paymentCoin: '',
        paymentCoinDivis: 8,
      },
      rejectConfirmOn: false,
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
  },
  computed: {
    heading () {
      if (ob.paymentNumber > 1) {
        return ob.polyT('orderDetail.summaryTab.payment.paymentHeading', {
          paymentNumber: ob.paymentNumber,
        });
      } else {
        return ob.polyT('orderDetail.summaryTab.payment.firstPaymentHeading');
      }
    },
    infoLine () {
      const priceFrag = ob.currencyMod.pairedCurrency(
        integerToDecimal(model.get('value'), _state.paymentCoinDivis),
        ob.paymentCoin,
        ob.userCurrency
      );

      let infoLine = '';

      if (ob.payee) {
        infoLine = ob.polyT(`orderDetail.summaryTab.payment.amountTo`, {
          currencyPairing: priceFrag,
          payeeName: ob.payee,
        });
      } else {
        // payee has not been set yet. It'll be set when the relevant profile is returned
        // asynchronously
        infoLine = priceFrag;
      }
      return infoLine;
    },
    confirmationsText () {
      let confirmationsText;

      if (this.confirmations < 10000) {
        confirmationsText = ob.polyT('orderDetail.summaryTab.payment.confirmationsCount', {
          smart_count: this.confirmations,
        });
      } else {
        confirmationsText = ob.polyT('orderDetail.summaryTab.payment.veryManyConfirmationsCount', {
          countText: ob.abbrNum(this.confirmations),
        });
      }
      return confirmationsText;
    },
    subText () {
      let subText = ob.polyT('orderDetail.summaryTab.payment.paidInFull');
      let roundedAmountShort = ob.amountShort;

      try {
        roundedAmountShort = ob.amountShort.dp(paymentCoinDivis);
      } catch (e) {
        // pass
      }

      if (ob.amountShort.gt(0)) {
        subText =
          ob.polyT('orderDetail.summaryTab.payment.underpaidAmountShort', {
            amountShort: roundedAmountShort,
          });
      }
      return subText;
    },
    confirmations () {
      const coinInfo = app.walletBalances.get(this._state.paymentCoin);

      let confirmations = 0;
      if (coinInfo && coinInfo.get('height') !== 0 && (+this.model.get('height'))) {
        confirmations = coinInfo.get('height') - this.model.get('height');
      }
      return confirmations;
    },
  },
  methods: {
    abbrNum,
    moment,

    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.boundOnDocClick = this.onDocumentClick.bind(this);
      $(document).on('click', this.boundOnDocClick);
    },

    onClickCancelOrder () {
      this.$emit('cancelClick', { view: this });
    },

    onClickAcceptOrder () {
      this.$emit('acceptClick', { view: this });
    },

    onClickRejectConfirmed () {
      this.$emit('confirmedRejectClick', { view: this });
      this.rejectConfirmOn = false;
    },

    onClickRejectOrder () {
      this.rejectConfirmOn = true;
      return false;
    },

    onClickRejectConfirmBox () {
      // ensure event doesn't bubble so onDocumentClick doesn't
      // close the confirmBox.
      return false;
    },

    onClickRejectConfirmCancel () {
      this.rejectConfirmOn = false;
    },

    onDocumentClick () {
      this.rejectConfirmOn = false;
    },

    remove () {
      $(document).off('click', this.boundOnDocClick);
    },
  }
}
</script>
<style lang="scss" scoped></style>
