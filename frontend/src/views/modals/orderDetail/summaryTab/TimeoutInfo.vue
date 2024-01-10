<template>
  <div class="timeoutInfo rowLg">

    <div class="headerIconWrap discSm clrP clrBr">
      <i class="ion-android-alarm-clock"></i>
    </div>
    <div class="innerContent border clrBr padMd">
      <div class="flexCol flexCent gutterVSm">
        <p :class="messageClass" v-html="message"></p>
        <div class="flexCent gutterH">
          <template v-if="ob.showDisputeBtn">
            <button class="btn tx5b clrErr clrBrDec1 clrTOnEmph " @click="onClickDisputeOrder">{{ ob.polyT('orderDetail.summaryTab.timeoutInfo.btnDisputeOrder') }}</button>
          </template>
          <template v-if="ob.ownPeerID === ob.vendor && ob.isPaymentClaimable">
            <ProcessingButton
              :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph tx5b js-claimPayment ${isClaimingPayment ? 'processing' : ''}`"
              :btnText="ob.polyT('orderDetail.summaryTab.timeoutInfo.btnClaimPayment')"
              @click="onClickClaimPayment"/>
          </template>
          <template v-if="ob.showDiscussBtn">
            <button class="btn tx5b clrP clrBr " @click="onClickDiscussOrder">{{ ob.polyT('orderDetail.summaryTab.timeoutInfo.btnDiscussOrder') }}</button>
          </template>
          <template v-if="ob.showResolveDisputeBtn">
            <ProcessingButton
              :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph tx5b js-resolveDispute ${ob.isResolvingDispute ? 'processing' : ''}`"
              :btnText="ob.polyT('orderDetail.summaryTab.timeoutInfo.btnResolveDispute')"
              @click="onClickResolveDispute"/>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import {
  releasingEscrow,
  releaseEscrow,
  events as orderEvents,
} from '../../../../../backbone/utils/order';
import { recordEvent } from '../../../../../backbone/utils/metrics';


export default {
  props: {
    options: {
      type: Object,
      default: {
        awaitingBlockHeight: false,
        isFundingConfirmed: false,
        isDisputed: false,
        hasDisputeEscrowExpired: false,
        canBuyerComplete: false,
        isPaymentClaimable: false,
        isPaymentFinalized: false,
        showDisputeBtn: false,
        showDiscussBtn: false,
        invalidContractData: false,
        dataUnavailable: false,
      },
    },
  },
  data () {
    return {
      isClaimingPayment: false,

      message: '',
      messageClass: 'txCtr',
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob(){
      return {
        ...this.templateHelpers,
        awaitingBlockHeight: false,
        isFundingConfirmed: false,
        isDisputed: false,
        hasDisputeEscrowExpired: false,
        canBuyerComplete: false,
        isPaymentClaimable: false,
        isPaymentFinalized: false,
        showDisputeBtn: false,
        showDiscussBtn: false,
        showResolveDisputeBtn: false,
        invalidContractData: false,
        dataUnavailable: false,
        ...this.options,
        ...this._state,
      };
    },
    isCase() {
      const ob = this.ob;
      return ob.ownPeerID !== ob.buyer && ob.ownPeerID !== ob.vendor;
    },
  },
  methods: {
    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide an orderID');
      }

      this.baseInit({
        initialState: {
          showResolveDisputeBtn: false,
          ...options.initialState,
        },
      });

      this.orderID = options.orderID;

      this.isClaimingPayment = releasingEscrow(this.orderID);
      this.listenTo(orderEvents, 'releasingEscrow', e => {
        if (e.id === this.orderID) {
          this.isClaimingPayment = true;
        }
      });

      this.listenTo(orderEvents, 'releaseEscrowComplete releaseEscrowFail', e => {
        if (e.id === this.orderID) {
          this.isClaimingPayment = false;
        }
      });

      this.listenTo(orderEvents, 'resolveDisputeComplete', () => {
        this.setState({
          showResolveDisputeBtn: false,
        });
      });

      this.updateMessageAndTip();
    },

    updateMessageAndTip() {
      let message;
      let messageClass = 'txCtr';
      let tip;
      let tipClass = '';

      const ob = this.ob;

      if (ob.invalidContractData) {
        if (ob.moderator) {
          message = ob.polyT('orderDetail.summaryTab.timeoutInfo.modInvalidContractData');
        } else {
          message = ob.isDisputed ?
            ob.polyT('orderDetail.summaryTab.timeoutInfo.disputedInvalidContractData') :
            ob.polyT('orderDetail.summaryTab.timeoutInfo.invalidContractData');
        }

        messageClass = 'txCtr clrTErr';
        tipClass = 'hide';
      } else if (ob.dataUnavailable) {
        if (ob.moderator) {
          message = ob.polyT('orderDetail.summaryTab.timeoutInfo.modDataUnavailable');
        } else {
          message = ob.isDisputed ?
            ob.polyT('orderDetail.summaryTab.timeoutInfo.disputedDataUnavailable') :
            ob.polyT('orderDetail.summaryTab.timeoutInfo.dataUnavailable');
        }

        messageClass = 'txCtr clrTErr';
        tipClass = 'hide';
      } else if (!this.isCase) {
        tip = ob.ownPeerID === ob.buyer ?
          ob.polyT('orderDetail.summaryTab.timeoutInfo.tipClaimAfterTimeoutBuyer') :
          ob.polyT('orderDetail.summaryTab.timeoutInfo.tipClaimAfterTimeoutVendor');

        if (ob.isDisputed) {
          if (!ob.hasDisputeEscrowExpired) {
            message = ob.polyT('orderDetail.summaryTab.timeoutInfo.disputedModOnClock', { timeRemaining: `<b>${ob.timeRemaining}</b>` });
            tip = ob.ownPeerID === ob.buyer ?
              ob.polyT('orderDetail.summaryTab.timeoutInfo.disputedTipBuyer', { totalTime: ob.totalTime }) :
              ob.polyT('orderDetail.summaryTab.timeoutInfo.disputedTipVendor', { totalTime: ob.totalTime });
          } else {
            message = ob.ownPeerID === ob.buyer ?
              ob.polyT('orderDetail.summaryTab.timeoutInfo.disputedTimeUpBuyer') :
              ob.polyT('orderDetail.summaryTab.timeoutInfo.disputedTimeUpVendor');
            tipClass = 'hide';
          }
        } else if (ob.isPaymentFinalized) {
          message =
            ob.polyT('orderDetail.summaryTab.timeoutInfo.paymentFinalizedVendor');        

          if (ob.ownPeerID === ob.buyer) {
            message = ob.canBuyerComplete ?
              ob.polyT('orderDetail.summaryTab.timeoutInfo.paymentFinalizedBuyerCompletable') :
              ob.polyT('orderDetail.summaryTab.timeoutInfo.paymentFinalizedBuyer');
          }
          tipClass = 'hide';
        } else if (!ob.isFundingConfirmed) {
          message = ob.polyT('orderDetail.summaryTab.timeoutInfo.unconfirmedFunding', {
            blocksRemaining: `<b>${ob.polyT('orderDetail.summaryTab.timeoutInfo.blocksRemaining',
              { blocksRemaining: ob.blocksRemaining })}</b>`,
            timeRemaining: `<b>${ob.timeRemaining}</b>`,
          });
        } else if (ob.blocksRemaining <= 0) {
          if (ob.ownPeerID === ob.buyer) {
            message = ob.polyT('orderDetail.summaryTab.timeoutInfo.escrowExpired');
            tip = ob.polyT('orderDetail.summaryTab.timeoutInfo.tipEscrowExpiredBuyer', { totalTime: ob.totalTime });
          } else {
            // it's the vendor
            if (ob.isPaymentClaimable) {
              message = ob.polyT('orderDetail.summaryTab.timeoutInfo.escrowExpiredVendorClaim');
              tip = ob.polyT('orderDetail.summaryTab.timeoutInfo.tipEscrowExpiredVendorClaim', { totalTime: ob.totalTime });
            } else {
              message = ob.polyT('orderDetail.summaryTab.timeoutInfo.escrowExpired');
              tipClass = 'hide';
            }
          }

          if (ob.ownPeerID === ob.buyer && ob.canBuyerComplete) {
            message = ob.polyT('orderDetail.summaryTab.timeoutInfo.escrowExpiredBuyerComplete');
          }
        } else {
          // time is left to open a dispute
          message = ob.polyT('orderDetail.summaryTab.timeoutInfo.escrowTimeRemaining', {
            blocksRemaining: `<b>${ob.polyT('orderDetail.summaryTab.timeoutInfo.blocksRemaining',
              { blocksRemaining: ob.blocksRemaining })}</b>`,
            timeRemaining: `<b>${ob.timeRemaining}</b>`,
          });
        }
      } else {
        // mod looking at case
        if (!ob.hasDisputeEscrowExpired) {
          message = ob.buyerOpened ?
            ob.polyT('orderDetail.summaryTab.timeoutInfo.modDisputeEscrowExpiredBuyerOpened',
              { timeRemaining: `<b>${ob.timeRemaining}</b>` }) :
            ob.polyT('orderDetail.summaryTab.timeoutInfo.modDisputeEscrowExpiredVendorOpened',
              { timeRemaining: `<b>${ob.timeRemaining}</b>` });
        } else {
          message = ob.polyT('orderDetail.summaryTab.timeoutInfo.modDisputeEscrowExpired');
        }

        tipClass = 'hide';
      }

      this.message = message + `<span class="toolTip clrT ${tipClass}" data-tip="${tip}"><i class="ion-help-circled"></i></span>`;
      this.messageClass = messageClass;
    },

    onClickDisputeOrder () {
      this.$emit('clickDisputeOrder');
    },

    onClickClaimPayment () {
      recordEvent('OrderDetails_TimeoutClaimPayment');
      releaseEscrow(this.orderID);
    },

    onClickDiscussOrder () {
      this.$emit('clickDiscussOrder');
    },

    onClickResolveDispute () {
      this.$emit('clickResolveDispute');
    },
  }
}
</script>
<style lang="scss" scoped></style>
