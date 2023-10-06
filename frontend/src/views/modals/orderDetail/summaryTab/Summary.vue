<template>
  <div class="summaryTab">
    <div class="flexHCent padLg">
      <div class="posR">
        <div class="clrT2 hide copiedToClipboard js-copiedToClipboard">{{ ob.polyT('copiedToClipboard') }}</div>
        <h1 class="inline tx4">{{ ob.polyT('orderDetail.summaryTab.orderNumber', { orderID: ob.id }) }}</h1>
        <a class="clrTEm tx5" @click="onClickCopyOrderID">{{ ob.polyT('orderDetail.summaryTab.copyLink') }}</a>
      </div>
    </div>

    <hr class="clrBr rowLg" />
    <div class="js-statusProgressBarContainer statusProgressBarContainer"></div>
    <div class="js-processingErrorContainer"></div>
    <hr class="clrBr rowLg" />

    <div class="js-timeoutInfoContainer"></div>
    <div class="js-subSections"></div>
    <template v-if="!ob.isCase">
      <div class="js-paymentsWrap"></div>
    </template>

    <template v-else>
      <div class="rowLg">
        <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.payment.firstPaymentHeading') }}</h2>
        <div class="border clrBr padMd">
          <template v-if="ob.blockChainAddressUrl">
            <a :href="ob.blockChainAddressUrl" class="clrTEm" v-html='ob.polyT("orderDetail.summaryTab.payment.viewPaymentDetails", {
                icon: `<span class="ion-android-open clrT2"></span>`,
              })'>
            </a>
          </template>

          <template v-else>
            <span class="clrTErr">{{ ob.polyT('orderDetail.summaryTab.unableToShowPayments') }}</span>
          </template>
        </div>
      </div>
    </template>
    <div class="js-payForOrderWrap payForOrderWrap rowLg border clrBr padMd"></div>
    <OrderDetails
      :options="{
        moderator,
      }"
      :bb="function() {
        return {
          model: contract,
        };
      }"/>
  </div>
</template>

<script>
import $ from 'jquery';
import moment from 'moment';
import { ipc } from '../../../../utils/ipcRenderer.js';
import app from '../../../../../backbone/app.js';
import 'velocity-animate';
import {
  completingOrder,
  events as orderEvents,
} from '../../../../../backbone/utils/order.js';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../../backbone/data/walletCurrencies.js';
import OrderCompletion from '../../../../../backbone/models/order/orderCompletion/OrderCompletion.js';
import { checkValidParticipantObject } from '../../../../utils/utils';
import StateProgressBar from '../../../../../backbone/views/modals/orderDetail/summaryTab/StateProgressBar';
import Payments from '../../../../../backbone/views/modals/orderDetail/summaryTab/Payments';
import Accepted from '../../../../../backbone/views/modals/orderDetail/summaryTab/Accepted';
import Fulfilled from '../../../../../backbone/views/modals/orderDetail/summaryTab/Fulfilled';
import Refunded from '../../../../../backbone/views/modals/orderDetail/summaryTab/Refunded';
import CompleteOrderForm from '../../../../../backbone/views/modals/orderDetail/summaryTab/CompleteOrderForm';
import OrderComplete from '../../../../../backbone/views/modals/orderDetail/summaryTab/OrderComplete';
import DisputeStarted from '../../../../../backbone/views/modals/orderDetail/summaryTab/DisputeStarted';
import DisputePayout from '../../../../../backbone/views/modals/orderDetail/summaryTab/DisputePayout';
import DisputeAcceptance from '../../../../../backbone/views/modals/orderDetail/summaryTab/DisputeAcceptance';
import TimeoutInfo from '../../../../../backbone/views/modals/orderDetail/summaryTab/TimeoutInfo';
import PayForOrder from '../../../../../backbone/views/modals/purchase/Payment';
import ProcessingError from '../../../../../backbone/views/modals/orderDetail/summaryTab/ProcessingError';

import OrderDetails from './OrderDetails.vue'

export default {
  components: {
    OrderDetails,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
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
    this.render();
  },
  computed: {
    ob() {
      const { paymentCoin } = this.model;
      let templateData = {
        ...this.templateHelpers,
        id: this.model.id,
        isCase: this.model.isCase,
        paymentCoin,
        ...this._model,
      };

      if (this.model.isCase) {
        const { paymentCoinData } = this.model;
        const { paymentAddress } = this;

        templateData = {
          ...templateData,
          blockChainAddressUrl: paymentCoinData
            ? paymentCoinData.getBlockChainAddressUrl(paymentAddress, app.serverConfig.testnet)
            : false,
          paymentAddress,
        };
      }

      return templateData;
    },
    progressBarState () {
      const orderState = this.model.get('state');
      const state = {
        states: [
          app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.paid'),
          app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.accepted'),
          app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.fulfilled'),
          app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.complete'),
        ],
      };

      if (orderState === 'DISPUTED' || orderState === 'DECIDED'
        || orderState === 'RESOLVED'
        || (['COMPLETED', 'PAYMENT_FINALIZED'].includes(orderState)
          && this.contract.get('disputeOpen') !== undefined)) {
        if (!this.model.isCase) {
          state.states = [
            app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.disputed'),
            app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.decided'),
            app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.resolved'),
          ];

          if (!this.model.vendorProcessingError) {
            // You can't complete an order and leave a review when the vendor had a processing error.
            // In that case the flow ends at resolved.
            state.states.push(
              app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.complete'),
            );
          }

          switch (orderState) {
            case 'DECIDED':
              state.currentState = 2;
              state.disputeState = 0;
              break;
            case 'RESOLVED':
              state.currentState = 3;
              state.disputeState = 0;
              break;
            case 'COMPLETED':
              state.currentState = 4;
              state.disputeState = 0;
              break;
            default:
              state.currentState = 1;
              state.disputeState = 1;
          }
        } else {
          state.states = [
            app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.disputed'),
            app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.complete'),
          ];

          switch (orderState) {
            case 'RESOLVED':
              state.currentState = 2;
              break;
            default:
              state.currentState = 1;
          }
        }
      } else if (['DECLINED', 'CANCELED', 'REFUNDED'].includes(orderState)) {
        state.states = [
          app.polyglot.t('orderDetail.summaryTab.orderDetails.progressBarStates.paid'),
          app.polyglot.t(
            `orderDetail.summaryTab.orderDetails.progressBarStates.${orderState.toLowerCase()}`,
          ),
        ];
        state.currentState = 2;
        state.disputeState = 0;
      } else {
        switch (orderState) {
          case 'PENDING':
            state.currentState = 1;
            break;
          case 'PARTIALLY_FULFILLED':
          case 'AWAITING_FULFILLMENT':
            state.currentState = 2;
            break;
          case 'FULFILLED':
          case 'AWAITING_PICKUP':
            state.currentState = 3;
            break;
          case 'COMPLETED':
            state.currentState = 4;
            break;
          case 'PAYMENT_FINALIZED':
            state.currentState = 1;

            if (this.contract.get('orderConfirmation')) {
              state.currentState = 2;
            }

            if (this.contract.get('orderFulfillments')) {
              state.currentState = 3;
            }

            break;
          default:
            state.currentState = 0;
        }
      }

      return state;
    },
    paymentAddress () {
      const vendorOrderConfirmation = this.contract.get('orderConfirmation');

      return (vendorOrderConfirmation && vendorOrderConfirmation.paymentAddress)
        || this.contract.get('orderOpen').payment.address;
    },
    shouldShowTimeoutInfoView () {
      const paymentCurData = this.model.paymentCoinData;

      return (
        (paymentCurData && paymentCurData.supportsEscrowTimeout)
        && (
          this.model.isOrderDisputable
          || ['DISPUTED', 'PAYMENT_FINALIZED'].includes(this.model.get('state'))
        )
      );
    },
    blockChainAddressUrl () {
      const { paymentCoinData } = this.model;
      return this.paymentCoinData
        ? paymentCoinData.getBlockChainAddressUrl(this.paymentAddress, app.serverConfig.testnet)
        : false;
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.contract = this.model.get('contract');

      if (this.model.isCase) {
        this.contract = this.model.get('disputeOpen').openedBy === 'BUYER'
          ? this.model.get('buyerContract')
          : this.model.get('vendorContract');
      }

      checkValidParticipantObject(options.buyer, 'buyer');
      checkValidParticipantObject(options.vendor, 'vendor');

      if (this.contract.get('orderOpen').payment.moderator) {
        checkValidParticipantObject(options.moderator, 'moderator');
      }

      this.vendor = options.vendor;
      this.buyer = options.buyer;
      this.moderator = options.moderator;

      this.listenTo(this.model, 'change:state', (md, state) => {
        this.stateProgressBar.setState(this.progressBarState);
        if (this.payments) this.payments.render();
        if (this.shouldShowAcceptedSection()) {
          if (!this.accepted) this.renderAcceptedView();
        } else if (this.accepted) {
          this.accepted.remove();
        }

        if (
          ['REFUNDED', 'FULFILLED', 'DISPUTED', 'DECIDED', 'RESOLVED', 'COMPLETED']
            .indexOf(state) > -1 && this.accepted) {
          const acceptedState = {
            showFulfillButton: false,
            infoText: app.polyglot.t('orderDetail.summaryTab.accepted.vendorReceived'),
            showRefundButton: false,
          };

          this.accepted.setState(acceptedState);
        }

        if (this.completeOrderForm
          && ['FULFILLED', 'RESOLVED'].indexOf(state) === -1) {
          this.completeOrderForm.remove();
          this.completeOrderForm = null;
        }

        if (state === 'PROCESSING_ERROR') {
          if (this.payForOrder && !this.shouldShowPayForOrderSection()) {
            this.payForOrder.remove();
            this.payForOrder = null;
          }
        }

        if (this.shouldShowCompleteOrderForm() && !this.completeOrderForm) {
          this.renderCompleteOrderForm();
        }

        this.renderProcessingError();
        this.renderTimeoutInfoView();
      });

      if (!this.model.isCase) {
        this.listenTo(this.contract, 'update:transactions', () => {
          if (this.payForOrder && !this.shouldShowPayForOrderSection()) {
            this.payForOrder.remove();
            this.payForOrder = null;
          }

          if (this.payments) {
            this.payments.collection.set(this.model.paymentsIn.models);
          }
        });

        this.listenTo(this.contract, 'change:refunds', () => this.renderRefundView());
      }

      this.listenTo(orderEvents, 'cancelOrderComplete', () => {
        this.model.set('state', 'CANCELED');
        // we'll refetch so our transaction list is updated with
        // the money returned to the buyer
        this.model.fetch();
      });

      this.listenTo(orderEvents, 'acceptOrderComplete', () => {
        // todo: factor in AWAITING_PICKUP
        this.model.set('state', 'AWAITING_FULFILLMENT');

        // we'll refetch so we get our vendorOrderConfirmation object
        this.model.fetch();
      });

      this.listenTo(orderEvents, 'rejectOrderComplete', () => {
        this.model.set('state', 'DECLINED');

        // We'll refetch so our transaction list is updated with
        // the money returned to the buyer (if they're online). If they're
        // not online the refund shows up when the buyer comes back online.
        this.model.fetch();
      });

      this.listenTo(this.contract, 'change:orderConfirmation', () => this.renderAcceptedView());

      this.listenTo(orderEvents, 'fulfillOrderComplete', (e) => {
        if (e.id === this.model.id) {
          this.model.set('state', 'FULFILLED');
          this.model.fetch();
        }
      });

      this.listenTo(orderEvents, 'refundOrderComplete', (e) => {
        if (e.id === this.model.id) {
          this.model.set('state', 'REFUNDED');
          this.model.fetch();
        }
      });

      this.listenTo(this.contract, 'change:orderFulfillments', () => {
        // For some reason the order state still reflects the order state at the
        // time this event handler is called even though it is triggered by fetch
        // which brings the updated order state in its payload. Weird... maybe
        // backbone doesn't update the model until the field specific change handlers
        // are called...? Anyways... the timeout below fixeds the issue.
        setTimeout(() => {
          this.renderFulfilledView();
        });
      });

      this.listenTo(this.contract, 'change:orderComplete', () => this.renderOrderCompleteView());

      this.listenTo(orderEvents, 'completeOrderComplete', (e) => {
        if (e.id === this.model.id && this.accepted) {
          this.model.set('state', 'COMPLETED');
          this.model.fetch();
        }
      });

      this.listenTo(orderEvents, 'openDisputeComplete', (e) => {
        if (e.id === this.model.id) {
          // The timeoutInfoView is expecting a dispute start time when
          // the order state is DISPUTED. Since we're setting the order state
          // now, but the server won't provide the dispute start time until
          // the fetch completes, we'll use a local dispute start time for
          // that brief gap.
          this.localDisputeStartTime = (new Date()).toISOString();
          this.listenToOnce(this.model, 'sync', () => {
            this.localDisputeStartTime = null;
          });
          this.model.fetch();
          this.model.set('state', 'DISPUTED');
        }
      });

      if (!this.model.isCase) {
        this.listenTo(this.contract, 'change:disputeOpen', () => this.renderDisputeStartedView());

        this.listenTo(this.contract, 'change:disputeClose', () => {
          // Only render the dispute payout the first time we receive it
          // (it changes from undefined to an object with data). It shouldn't
          // be changing after that, but for some reason it is.
          if (!this.contract.previous('disputeClose')) {
            // The timeout is needed in the handler so the updated
            // order state is available.
            setTimeout(() => this.renderDisputePayoutView());
          }
        });

        this.listenTo(orderEvents, 'acceptPayoutComplete', (e) => {
          if (e.id === this.model.id) {
            this.model.set('state', 'RESOLVED');
            this.model.fetch();
          }
        });

        this.listenTo(this.contract, 'change:disputeAccept', () => {
          this.renderDisputeAcceptanceView();

          if (this.disputePayout) {
            this.disputePayout.setState({ showAcceptButton: false });
          }
        });
      } else {
        this.listenTo(orderEvents, 'resolveDisputeComplete', (e) => {
          if (e.id === this.model.id) {
            this.model.set('state', 'RESOLVED');
            this.model.fetch();
          }
        });

        this.listenTo(this.model, 'change:disputeClose', () => this.renderDisputePayoutView());
      }

      this.listenTo(orderEvents, 'releaseEscrowComplete', (e) => {
        if (e.id === this.model.id) {
          this.model.set('state', 'PAYMENT_FINALIZED');
          this.model.fetch();
        }
      });

      const balanceMd = app.walletBalances.get(this.model.paymentCoin);
      const bindHeightChange = (md) => {
        this.listenTo(md, 'change:height', () => {
          if (this.timeoutInfo || this.shouldShowTimeoutInfoView) {
            this.renderTimeoutInfoView();
          }
        });
      };

      if (balanceMd) {
        bindHeightChange(balanceMd);
      } else {
        this.listenTo(app.walletBalances, 'add', (md) => {
          if (md.id === this.model.paymentCoin) {
            bindHeightChange(md);
          }
        });
      }
    },

    onClickCopyOrderID () {
      ipc.send('controller.system.writeToClipboard', this.model.id);
      this.copiedToClipboardAnimatingIn = true;
      $('.js-copiedToClipboard')
        .velocity('stop')
        .velocity('fadeIn', {
          complete: () => {
            this.$copiedToClipboard
              .velocity('fadeOut', { delay: 1000 });
          },
        });
    },

    setDisputeCountdownTimeout (...args) {
      clearTimeout(this.disputeCountdownTimeout);
      this.disputeCountdownTimeout = setTimeout(...args);
    },


    renderTimeoutInfoView () {
      const paymentCurData = this.model.paymentCoinData;
      const orderState = this.model.get('state');
      const prevMomentDaysThreshold = moment.relativeTimeThreshold('d');
      const { isCase } = this.model;

      if (!this.shouldShowTimeoutInfoView) {
        if (this.timeoutInfo) this.timeoutInfo.remove();
        this.timeoutInfo = null;
        clearTimeout(this.disputeCountdownTimeout);
        return;
      }

      // temporarily upping the moment threshold of number of days before month is used,
      // so in the escrow timeouts 45 is represented as '45 days' instead of '1 month'.
      moment.relativeTimeThreshold('d', 364);

      let state = {
        ownPeerID: app.profile.id,
        buyer: this.buyer.id,
        vendor: this.vendor.id,
        moderator: (this.moderator && this.moderator.id) || undefined,
        isFundingConfirmed: false,
        blockTime: paymentCurData && paymentCurData.blockTime,
        isDisputed: orderState === 'DISPUTED',
        hasDisputeEscrowExpired: false,
        canBuyerComplete: this.model.canBuyerComplete,
        isPaymentClaimable: false,
        isPaymentFinalized: false,
        showDisputeBtn: false,
        showDiscussBtn: orderState === 'DISPUTED',
        showResolveDisputeBtn: false,
        dataUnavailable: false,
      };

      if (orderState === 'PAYMENT_FINALIZED') {
        state.isPaymentFinalized = true;
      } else {
        let disputeStartTime;
        let escrowTimeoutHours;
        let curHeight;

        try {
          escrowTimeoutHours = this.contract.escrowTimeoutHours;
        } catch (e) {
          // pass - will be handled below
        }

        try {
          curHeight = app.walletBalances
            .get(this.model.paymentCoin)
            .get('height');
        } catch (e) {
          // pass
        }

        if (orderState === 'DISPUTED' || isCase) {
          try {
            if (isCase) {
              disputeStartTime = this.model.get('timestamp');
            } else {
              disputeStartTime = this.localDisputeStartTime
                || this.contract.get('disputeOpen').timestamp;
            }
          } catch (e) {
            throw e;
            // pass - will be handled below
          }
        }

        console.log(this.model.contract);

        if (
          (orderState !== 'DISPUTED' && !escrowTimeoutHours)
          || (this.model.contract.dispute !== undefined)
          || (orderState === 'DISPUTED' && !Date.parse(disputeStartTime))
        ) {
          // contract probably forged
          state = {
            ...state,
            invalidContractData: true,
            showDisputeBtn: this.model.isOrderStateDisputable,
            showResolveDisputeBtn: isCase,
          };
        } else if (!paymentCurData || !curHeight) {
          // The order was paid in a coin not supported by this client or we don't have
          // the current height of the paymentCoin, which means we don't know the
          // blocktime and can't display timeout info.
          state = {
            dataUnavailable: true,
          };
        } else {
          const timeoutHours = orderState === 'DISPUTED'
            ? this.contract.disputeExpiry : escrowTimeoutHours;
          let hasDisputeEscrowExpired;
          const totalMs = timeoutHours * 60 * 60 * 1000;
          state.totalTime = moment(Date.now()).from(moment(Date.now() + totalMs), true);

          if (isCase || orderState === 'DISPUTED') {
            const msSinceDisputeStart = Date.now() - (new Date(disputeStartTime)).getTime();
            const msRemaining = totalMs - msSinceDisputeStart;
            hasDisputeEscrowExpired = msRemaining <= 0;

            state = {
              ...state,
              hasDisputeEscrowExpired,
              timeRemaining: hasDisputeEscrowExpired ? 0
                : moment(Date.now()).from(moment(Date.now() + msRemaining), true),
              showDiscussBtn: !hasDisputeEscrowExpired,
            };

            if (!hasDisputeEscrowExpired) {
              let checkBackInMs = 1000; // every second

              if (msRemaining > 1000 * 60 * 60 * 24) {
                // greater than a day
                checkBackInMs = 1000 * 60 * 60 * 20;
              } else if (msRemaining > 1000 * 60 * 60) {
                // greater than a hour
                checkBackInMs = 1000 * 60 * 55;
              } else if (msRemaining > 1000 * 60) {
                // greater than 1 minute
                checkBackInMs = 5000;
              }

              this.setDisputeCountdownTimeout(
                () => this.renderTimeoutInfoView(),
                checkBackInMs,
              );
            }
          }

          if (isCase) {
            state = {
              ...state,
              buyerOpened: this.model.get('buyerOpened'),
              showResolveDisputeBtn: !hasDisputeEscrowExpired,
            };
          } else if (orderState === 'DISPUTED') {
            state = {
              ...state,
              isPaymentClaimable: hasDisputeEscrowExpired,
            };
          } else {
            const fundedHeight = this.model.fundedBlockHeight;
            const blocksPerTimeout = (timeoutHours * 60 * 60 * 1000) / paymentCurData.blockTime;
            const blocksRemaining = fundedHeight
              ? blocksPerTimeout - (curHeight - fundedHeight)
              : blocksPerTimeout;
            const msRemaining = blocksRemaining * paymentCurData.blockTime;

            const timeRemaining = moment(Date.now()).from(moment(Date.now() + msRemaining), true);

            state = {
              ...state,
              isFundingConfirmed: !!fundedHeight,
              blocksRemaining,
              timeRemaining,
              showDisputeBtn: this.model.isOrderDisputable && blocksRemaining > 0,
              isPaymentClaimable: orderState === 'FULFILLED' && blocksRemaining <= 0,
            };
          }
        }
      }

      // restore the days timeout threshold
      moment.relativeTimeThreshold('d', prevMomentDaysThreshold);

      if (this.timeoutInfo) {
        this.timeoutInfo.setState(state);
      } else {
        this.timeoutInfo = this.createChild(TimeoutInfo, {
          orderID: this.model.id,
          initialState: state,
        });

        $('.js-timeoutInfoContainer').html(this.timeoutInfo.render().el);

        this.listenTo(this.timeoutInfo, 'clickDisputeOrder', () => this.$emit('clickDisputeOrder'));

        this.listenTo(this.timeoutInfo, 'clickDiscussOrder', () => this.$emit('clickDiscussOrder'));

        this.listenTo(this.timeoutInfo, 'clickResolveDispute', () => this.trigger('clickResolveDispute'));
      }
    },

    shouldShowPayForOrderSection () {
      return (
        this.buyer.id === app.profile.id
        && !this.model.isFunded
        && !this.model.vendorProcessingError
      );
    },

    shouldShowAcceptedSection () {
      let bool = false;

      // Show the accepted section if the order has been accepted and its fully funded.
      if (this.contract.get('orderConfirmation')
        && (this.model.isCase || this.model.isFunded)) {
        bool = true;
      }

      return bool;
    },

    renderAcceptedView () {
      const vendorOrderConfirmation = this.contract.get('orderConfirmation');

      if (!vendorOrderConfirmation) {
        throw new Error('Unable to create the accepted view because the vendorOrderConfirmation '
          + 'data object has not been set.');
      }

      const orderState = this.model.get('state');
      const isVendor = this.vendor.id === app.profile.id;
      const canFulfill = isVendor && [
        'AWAITING_FULFILLMENT',
        'PARTIALLY_FULFILLED',
      ].indexOf(orderState) > -1;
      const initialState = {
        timestamp: vendorOrderConfirmation.timestamp,
        showRefundButton: isVendor && [
          'AWAITING_FULFILLMENT',
          'PARTIALLY_FULFILLED',
        ].indexOf(orderState) > -1,
        showFulfillButton: canFulfill,
        paymentCoin: this.model.paymentCoin,
      };

      if (!this.model.isCase) {
        if (isVendor) {
          // vendor looking at the order
          if (canFulfill) {
            initialState.infoText = app.polyglot.t('orderDetail.summaryTab.accepted.vendorCanFulfill');
          } else {
            initialState.infoText = app.polyglot.t('orderDetail.summaryTab.accepted.vendorReceived');
          }
        } else {
          // buyer looking at the order
          initialState.infoText = app.polyglot.t('orderDetail.summaryTab.accepted.buyerOrderAccepted');
        }
      } else {
        // mod looking at the order
        initialState.infoText = app.polyglot.t('orderDetail.summaryTab.accepted.modOrderAccepted');
      }

      if (this.accepted) this.accepted.remove();
      this.accepted = this.createChild(Accepted, {
        orderID: this.model.id,
        initialState,
      });
      this.listenTo(this.accepted, 'clickFulfillOrder', () => this.$emit('clickFulfillOrder'));

      this.vendor.getProfile()
        .done((profile) => {
          this.accepted.setState({
            avatarHashes: profile.get('avatarHashes').toJSON(),
          });
        });

      $('.js-subSections').prepend(this.accepted.render().el);
    },

    renderRefundView () {
      const refundMd = this.contract.get('refunds')[0];

      if (this.refunded) this.refunded.remove();

      if (!refundMd) {
        console.error('Unable to create the refunded view because the refunds '
          + 'data object has not been set.');
        return;
      }

      const { paymentCoin } = this.model;

      let blockChainTxUrl = false;

      try {
        blockChainTxUrl = getWalletCurByCode(paymentCoin)
          .getBlockChainTxUrl(refundMd.transactionID, app.serverConfig.testnet);
      } catch (e) {
        // pass
      }

      let height = 0;
      const transaction = this.contract.get('transactions').find((tx) => tx.txid === refundMd.transactionID);
      if (transaction) {
        height = +transaction.height;
      }

      const coinInfo = app.walletBalances.get(paymentCoin);
      let confirmations = 0;
      if (coinInfo.get('height') !== 0 && height) {
        confirmations = coinInfo.get('height') - height;
      }

      this.refunded = this.createChild(Refunded, {
        model: refundMd,
        initialState: {
          isCrypto: this.contract.type === 'CRYPTOCURRENCY',
          blockChainTxUrl,
          paymentCoin,
          confirmations,
        },
      });
      this.buyer.getProfile()
        .done((profile) => this.refunded.setState({ buyerName: profile.get('name') }));
      $('.js-subSections').prepend(this.refunded.render().el);
    },

    shouldShowCompleteOrderForm () {
      return this.buyer.id === app.profile.id
        && this.model.canBuyerComplete;
    },

    renderCompleteOrderForm () {
      const completingObject = completingOrder(this.model.id);
      const model = new OrderCompletion(
        completingObject ? completingObject.data : { orderID: this.model.id },
      );
      if (this.completeOrderForm) this.completeOrderForm.remove();
      this.completeOrderForm = this.createChild(CompleteOrderForm, {
        model,
        slug: this.contract.get('orderOpen').listings[0].listing.slug,
      });

      $('.js-subSections').prepend(this.completeOrderForm.render().el);
    },

    renderFulfilledView () {
      const data = this.contract.get('orderFulfillments');

      if (!data) {
        throw new Error('Unable to create the fulfilled view because the vendorOrderFulfillment '
          + 'data object has not been set.');
      }

      const fulfilledState = {
        contractType: this.contract.type,
        showPassword: (this.moderator && this.moderator.id !== app.profile.id) || true,
        isLocalPickup: this.contract.isLocalPickup,
      };

      if (this.contract.type === 'CRYPTOCURRENCY') {
        fulfilledState.coinType = this.contract.get('orderOpen').listings[0].listing.metadata.coinType;
      }

      if (this.fulfilled) this.fulfilled.remove();
      this.fulfilled = this.createChild(Fulfilled, {
        dataObject: data[0],
        initialState: fulfilledState,
      });

      if (app.profile.id === this.vendor.id) {
        this.fulfilled.setState({
          noteFromLabel: app.polyglot.t('orderDetail.summaryTab.fulfilled.yourNoteLabel'),
        });
      } else {
        this.vendor.getProfile()
          .done((profile) => {
            this.fulfilled.setState({
              noteFromLabel: app.polyglot.t('orderDetail.summaryTab.fulfilled.noteFromStoreLabel', { store: profile.get('name') }),
            });
          });
      }

      if (this.completeOrderForm) {
        this.completeOrderForm.$el.after(this.fulfilled.render().el);
      } else {
        $('.js-subSections').prepend(this.fulfilled.render().el);

        if (this.shouldShowCompleteOrderForm()) this.renderCompleteOrderForm();
      }
    },

    renderOrderCompleteView () {
      const data = this.contract.get('orderComplete');

      if (!data) {
        throw new Error('Unable to create the Order Complete view because the buyerOrderCompletion '
          + 'data object has not been set.');
      }

      if (this.completeOrderForm) {
        this.completeOrderForm.remove();
        this.completeOrderForm = null;
      }

      if (this.orderComplete) this.orderComplete.remove();
      this.orderComplete = this.createChild(OrderComplete, {
        dataObject: data,
      });

      this.buyer.getProfile()
        .done((profile) => this.orderComplete.setState({ buyerName: profile.get('name') }));
      $('.js-subSections').prepend(this.orderComplete.render().el);
    },

    renderDisputeStartedView () {
      const data = this.model.isCase ? {
        timestamp: this.model.get('timestamp'),
        claim: this.model.get('claim'),
      } : this.contract.get('disputeOpen');

      if (!data) {
        throw new Error('Unable to create the Dispute Started view because the dispute '
          + 'data object has not been set.');
      }

      let paymentCoinData;

      try {
        paymentCoinData = getWalletCurByCode(this.model.paymentCoin);
      } catch (e) {
        // pass
      }

      if (this.disputeStarted) this.disputeStarted.remove();
      this.disputeStarted = this.createChild(DisputeStarted, {
        initialState: {
          ...data,
          showResolveButton: this.model.get('state') === 'DISPUTED'
            && this.model.isCase
            && (!paymentCoinData || !paymentCoinData.supportsEscrowTimeout),
        },
      });

      // this is only set on the Case.
      const buyerOpened = this.model.get('buyerOpened');
      if (typeof buyerOpened !== 'undefined') {
        const disputeOpener = buyerOpened ? this.buyer : this.vendor;
        disputeOpener.getProfile()
          .done((profile) => this.disputeStarted.setState({ disputerName: profile.get('name') }));
      }

      this.listenTo(this.disputeStarted, 'clickResolveDispute', () => this.$emit('clickResolveDispute'));

      $('.js-subSections').prepend(this.disputeStarted.render().el);
    },

    renderDisputePayoutView () {
      const data = this.model.isCase ? this.model.get('disputeClose')
        : this.contract.get('disputeClose');

      if (!data) {
        throw new Error('Unable to create the Dispute Payout view because the resolution '
          + 'data object has not been set.');
      }

      if (this.disputePayout) this.disputePayout.remove();
      this.disputePayout = this.createChild(DisputePayout, {
        orderID: this.model.id,
        initialState: {
          ...data,
          showAcceptButton: !this.model.isCase && this.model.get('state') === 'DECIDED',
          paymentCoin: this.model.paymentCoin,
        },
      });

      ['buyer', 'vendor', 'moderator'].forEach((type) => {
        this[type].getProfile().done((profile) => {
          const state = {};
          state[`${type}Name`] = profile.get('name');
          state[`${type}AvatarHashes`] = profile.get('avatarHashes').toJSON();
          this.disputePayout.setState(state);
        });
      });

      $('.js-subSections').prepend(this.disputePayout.render().el);
    },

    renderPayForOrder () {
      const { paymentCoin } = this.model;

      if (getWalletCurByCode(paymentCoin)) {
        if (this.payForOrder) this.payForOrder.remove();

        this.payForOrder = this.createChild(PayForOrder, {
          balanceRemaining: this.model.getBalanceRemaining(),
          paymentAddress: this.paymentAddress,
          orderID: this.model.id,
          isModerated: !!this.moderator,
          metricsOrigin: 'Transactions',
          paymentCoin: this.model.paymentCoin,
        });

        this.getCachedEl('.js-payForOrderWrap').html(this.payForOrder.render().el);
      }
    },

    renderDisputeAcceptanceView () {
      const data = this.contract.get('disputeAccept');

      if (!data) {
        throw new Error('Unable to create the Dispute Acceptance view because the '
          + 'disputeAccept data object has not been set.');
      }

      const closer = data.closedBy
        === this.buyer.id ? this.buyer : this.vendor;

      if (this.disputeAcceptance) this.disputeAcceptance.remove();
      this.disputeAcceptance = this.createChild(DisputeAcceptance, {
        initialState: {
          timestamp: data.timestamp,
          acceptedByBuyer: closer.id === this.buyer.id,
          buyerViewing: app.profile.id === this.buyer.id,
          vendorProcessingError: this.model.vendorProcessingError,
        },
      });

      closer.getProfile()
        .done((profile) => this.disputeAcceptance.setState({
          closerName: profile.get('name'),
          closerAvatarHashes: profile.get('avatarHashes').toJSON(),
        }));

      if (this.completeOrderForm) {
        this.completeOrderForm.$el.after(this.disputeAcceptance.render().el);
      } else {
        $('.js-subSections').prepend(this.disputeAcceptance.render().el);

        if (this.shouldShowCompleteOrderForm()) this.renderCompleteOrderForm();
      }
    },

    /**
     * Will render sub-sections in order based on their timestamp. Exempt from
     * this are the Order Details, Payment Details and Accepted sections which
     * are always first and in a specific order.
     */
    renderSubSections () {
      const sections = [];
      const { isCase } = this.model;

      if (this.contract.get('refunds').length > 0) {
        sections.push({
          function: this.renderRefundView,
          timestamp:
            (new Date(this.contract.get('refunds')[0].timestamp)),
        });
      }

      if (this.contract.get('orderFulfillments') && this.contract.get('orderFulfillments').length > 0) {
        sections.push({
          function: this.renderFulfilledView,
          timestamp:
            (new Date(this.contract.get('orderFulfillments')[0].timestamp)),
        });
      }

      if (this.contract.get('orderComplete')) {
        sections.push({
          function: this.renderOrderCompleteView,
          timestamp:
            (new Date(this.contract.get('orderComplete').timestamp)),
        });
      }

      if (this.contract.get('disputeOpen') || isCase) {
        const timestamp = isCase
          ? this.model.get('timestamp')
          : this.contract.get('disputeOpen').timestamp;

        sections.push({
          function: this.renderDisputeStartedView,
          timestamp:
            (new Date(timestamp)),
        });
      }

      if (this.contract.get('disputeClose')
        || (isCase && this.model.get('disputeClose'))) {
        const timestamp = isCase
          ? this.model.get('disputeClose').timestamp
          : this.contract.get('disputeClose').timestamp;

        sections.push({
          function: this.renderDisputePayoutView,
          timestamp:
            (new Date(timestamp)),
        });
      }

      if (this.contract.get('disputeAccept')) {
        sections.push({
          function: this.renderDisputeAcceptanceView,
          timestamp:
            (new Date(this.contract.get('disputeAccept').timestamp)),
        });
      }

      sections.sort((a, b) => (a.timestamp - b.timestamp))
        .forEach((section) => {
          if (typeof section.function === 'function') {
            section.function.call(this);
          } else {
            throw new Error('Unable to add sub section. It doesn\'t have a creation function.');
          }
        });
    },

    renderProcessingError () {
      if (!this.model.vendorProcessingError) {
        if (this.processingError) {
          this.processingError.remove();
          this.processingError = null;
        }

        return;
      }

      const isBuyer = this.buyer.id === app.profile.id;
      const state = {
        isBuyer,
        isModerator: !!(this.moderator && this.moderator.id),
        isOrderCancelable: this.model.isOrderCancelable,
        isModerated: !!this.moderator,
        isCase: this.model.isCase,
        isDisputable: isBuyer
          && this.model.isOrderDisputable
          && this.model.get('state') === 'PROCESSING_ERROR',
        errors: this.contract.get('erroredMessages') || [],
      };

      if (!this.processingError) {
        this.processingError = this.createChild(ProcessingError, {
          orderID: this.model.id,
          initialState: state,
        });
        this.getCachedEl('.js-processingErrorContainer')
          .html(this.processingError.render().el);
      } else {
        this.processingError.setState(state);
      }
    },

    remove () {
      clearTimeout(this.disputeCountdownTimeout);
    },

    render () {
      const { paymentCoin } = this.model;


      if (this.stateProgressBar) this.stateProgressBar.remove();
      this.stateProgressBar = this.createChild(StateProgressBar, {
        initialState: this.progressBarState,
      });
      $('.js-statusProgressBarContainer').html(this.stateProgressBar.render().el);

      if (this.shouldShowPayForOrderSection()) {
        this.renderPayForOrder();
      }

      this.renderTimeoutInfoView();

      if (!this.model.isCase) {
        if (this.payments) this.payments.remove();
        this.payments = this.createChild(Payments, {
          orderID: this.model.id,
          collection: this.model.paymentsIn,
          orderPrice: this.model.orderPrice,
          paymentCoin,
          vendor: this.vendor,
          isOrderCancelable: () => this.model.isOrderCancelable,
          isCrypto: this.contract.type === 'CRYPTOCURRENCY',
          isOrderConfirmable: () => this.model.get('state') === 'PENDING'
            && this.vendor.id === app.profile.id && !this.contract.get('orderConfirmation'),
          // paymentCoin,
        });
        $('.js-paymentsWrap').html(this.payments.render().el);
      }

      if (this.shouldShowAcceptedSection()) this.renderAcceptedView();
      this.renderSubSections();
      this.renderProcessingError();

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
