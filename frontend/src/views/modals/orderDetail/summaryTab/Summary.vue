<template>
  <div class="summaryTab">
    <div class="flexHCent padLg">
      <div class="posR">
        <div class="clrT2 hide copiedToClipboard js-copiedToClipboard">{{ ob.polyT('copiedToClipboard') }}</div>
        <h1 class="inline tx4">{{ ob.polyT('orderDetail.summaryTab.orderNumber', { orderID: ob.id }) }}</h1>
        <a class="clrTEm tx5" @click="onClickCopyOrderID">{{ ' ' + ob.polyT('orderDetail.summaryTab.copyLink') }}</a>
      </div>
    </div>

    <hr class="clrBr rowLg" />
    <div class="js-statusProgressBarContainer statusProgressBarContainer">
      <StateProgressBar :barState="progressBarState" />
    </div>
    <div class="js-processingErrorContainer">
      <ProcessingError ref="processingError" v-if="showProcessingError" :options="processingErrorOptions" />
    </div>
    <hr class="clrBr rowLg" />

    <div class="js-timeoutInfoContainer">
      <TimeoutInfo ref="timeoutInfo" v-if="showTimeoutInfo" :options="timeoutInfoOptions"
        @clickDisputeOrder="$emit('clickDisputeOrder')"
        @clickDiscussOrder="$emit('clickDiscussOrder')"
        @clickResolveDispute="$emit('clickResolveDispute')"
      />
    </div>
    <div class="js-subSections">
      <CompleteOrderForm ref="completeOrderForm" v-if="showCompleteOrderForm"
        :options="{
          orderID: model.id,
          slug: contract.get('orderOpen').listings[0].listing.slug,
        }"/>
      <OrderComplete ref="orderComplete" v-if="showOrderComplete" :dataObject="contract.get('orderComplete')" />
      <DisputePayout ref="disputePayout" v-if="showDisputePayout" :options="{
        orderID: model.id,
        initialState: {
          ...(model.isCase ? model.get('disputeClose') : contract.get('disputeClose')),
          showAcceptButton: !model.isCase && model.get('state') === 'DECIDED',
          paymentCoin: model.paymentCoin,
        },
      }" />
      <DisputeAcceptance ref="disputeAcceptance" v-if="showDisputeAcceptance" :options="disputeAcceptanceOptions" />
      <DisputeStarted ref="disputeStarted" v-if="showDisputeStarted" :options="disputeStartedOptions" @clickResolveDispute="$emit('clickResolveDispute')" />
      <Fulfilled ref="fulfilled" v-if="showFulfilled" :options="fulfilledOptions" />
      <Accepted ref="accepted" v-if="showAccepted" :options="acceptedOptions" @clickFulfillOrder="$emit('clickFulfillOrder')" />
      <Refunded ref="refunded" v-if="showRefunded" :options="refundedOptions"/>
    </div>
    <template v-if="!ob.isCase">
      <div ref="paymentsWrap" class="js-paymentsWrap">
        <Payments :key="paymentsInKey" :options="{
          orderID: model.id,
          collection: model.paymentsIn,
          orderPrice: model.orderPrice,
          paymentCoin: model.paymentCoin,
          vendor,
          isOrderCancelable: () => model.isOrderCancelable,
          isCrypto: contract.type === 'CRYPTOCURRENCY',
          isOrderConfirmable: () => model.get('state') === 'PENDING' && isVendor && !contract.get('orderConfirmation'),
          // paymentCoin,
        }" />
      </div>
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
    <div class="js-payForOrderWrap payForOrderWrap rowLg border clrBr padMd">
      <PayForOrder
        v-if="showPayForOrder"
        :options="{
          balanceRemaining: model.getBalanceRemaining(),
          paymentAddress: paymentAddress,
          orderID: model.id,
          isModerated: !!moderator,
          metricsOrigin: 'Transactions',
          paymentCoin: model.paymentCoin,
        }"
      />
    </div>
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
  events as orderEvents,
} from '../../../../../backbone/utils/order.js';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../../backbone/data/walletCurrencies.js';
import { checkValidParticipantObject } from '../../../../utils/utils';

import StateProgressBar from './StateProgressBar.vue';
import TimeoutInfo from './TimeoutInfo.vue';
import PayForOrder from '../../purchase/Payment.vue';
import Accepted from './Accepted.vue';
import Refunded from './Refunded.vue';
import CompleteOrderForm from './CompleteOrderForm.vue';
import Payments from './Payments.vue';
import Fulfilled from './Fulfilled.vue';
import OrderComplete from './OrderComplete.vue';
import DisputeStarted from './DisputeStarted.vue';
import DisputePayout from './DisputePayout.vue';
import DisputeAcceptance from './DisputeAcceptance.vue';
import OrderDetails from './OrderDetails.vue';
import ProcessingError from './ProcessingError.vue';

export default {
  components: {
    StateProgressBar,
    TimeoutInfo,
    PayForOrder,
    Accepted,
    Refunded,
    CompleteOrderForm,
    Payments,
    Fulfilled,
    OrderComplete,
    DisputeStarted,
    DisputePayout,
    DisputeAcceptance,
    OrderDetails,
    ProcessingError,
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
      moderator: undefined,

      showTimeoutInfo: false,
      timeoutInfoOptions: {},

      showPayForOrder: false,

      showAccepted: false,
      acceptedOptions: {},

      showRefunded: false,
      refundedOptions: {},

      showCompleteOrderForm: false,

      paymentsInKey: 0,

      showFulfilled: false,
      fulfilledOptions: {},

      showOrderComplete: false,
      orderCompleteOptions: {},

      showDisputeStarted: false,
      disputeStartedOptions: {},

      showDisputePayout: false,

      showDisputeAcceptance: false,
      disputeAcceptanceOptions: {},

      showProcessingError: false,
      processingErrorOptions: {},
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
        ...this.model.toJSON(),
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
    },
    isVendor() {
      return this.vendor.id === app.profile.id;
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

      this.listenTo(this.model, 'change:state', (md, state) => {
        this.paymentsInKey += 1;

        if (this.shouldShowAcceptedSection()) {
          if (!this.showAccepted) this.renderAcceptedView();
        } else if (this.showAccepted) {
          this.showAccepted = false;
        }

        if (
          ['REFUNDED', 'FULFILLED', 'DISPUTED', 'DECIDED', 'RESOLVED', 'COMPLETED']
            .indexOf(state) > -1 && this.showAccepted) {
          this.$nextTick(() => {
            const acceptedState = {
              showFulfillButton: false,
              infoText: app.polyglot.t('orderDetail.summaryTab.accepted.vendorReceived'),
              showRefundButton: false,
            };

            this.$refs.accepted && this.$refs.accepted.setState(acceptedState);
          })
        }

        if (['FULFILLED', 'RESOLVED'].indexOf(state) === -1) {
            this.showCompleteOrderForm = false;
        }

        if (state === 'PROCESSING_ERROR') {
          if (this.showPayForOrder && !this.shouldShowPayForOrderSection()) {
            this.showPayForOrder = false;
          }
        }

        if (this.shouldShowCompleteOrderForm()) {
          this.showCompleteOrderForm = true;
        }

        this.renderProcessingError();
        this.renderTimeoutInfoView();
      });

      if (!this.model.isCase) {
        this.listenTo(this.contract, 'update:transactions', () => {
          if (this.showPayForOrder && !this.shouldShowPayForOrderSection()) {
            this.showPayForOrder = false;
          }

          this.paymentsInKey += 1;
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
        if (e.id === this.model.id && this.showAccepted) {
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

          if (this.$refs.disputePayout) {
            this.$refs.disputePayout.setState({ showAcceptButton: false });
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
          if (this.showTimeoutInfo || this.shouldShowTimeoutInfoView) {
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
        this.showTimeoutInfo = false;

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

      if (this.showTimeoutInfo) {
        this.$refs.timeoutInfo.setState(state);
      } else {
        this.timeoutInfoOptions = {
          orderID: this.model.id,
          initialState: state,
        };
        this.showTimeoutInfo = true;
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
      const canFulfill = this.isVendor && [
        'AWAITING_FULFILLMENT',
        'PARTIALLY_FULFILLED',
      ].indexOf(orderState) > -1;
      const initialState = {
        timestamp: vendorOrderConfirmation.timestamp,
        showRefundButton: this.isVendor && [
          'AWAITING_FULFILLMENT',
          'PARTIALLY_FULFILLED',
        ].indexOf(orderState) > -1,
        showFulfillButton: canFulfill,
        paymentCoin: this.model.paymentCoin,
      };

      if (!this.model.isCase) {
        if (this.isVendor) {
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

      this.acceptedOptions = {
        orderID: this.model.id,
        initialState,
      }
      this.showAccepted = true;

      this.$nextTick(() => {
        this.vendor.getProfile()
          .done((profile) => {
            this.$refs.accepted && this.$refs.accepted.setState({
              avatarHashes: profile.get('avatarHashes').toJSON(),
            });
          });
      })
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

      this.refundedOptions = {
        model: refundMd,
        initialState: {
          isCrypto: this.contract.type === 'CRYPTOCURRENCY',
          blockChainTxUrl,
          paymentCoin,
          confirmations,
        },
      };
      this.showRefunded = true;

      this.$nextTick(() => {
        this.buyer.getProfile()
          .done((profile) => this.$refs.refunded && this.$refs.refunded.setState({ buyerName: profile.get('name') }));
      })
    },

    shouldShowCompleteOrderForm () {
      return this.buyer.id === app.profile.id
        && this.model.canBuyerComplete;
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

      this.fulfilledOptions = {
        dataObject: data[0],
        initialState: fulfilledState,
      };
      this.showFulfilled = true;

      this.$nextTick(() => {
        if (app.profile.id === this.vendor.id) {
          this.$refs.fulfilled && this.$refs.fulfilled.setState({
            noteFromLabel: app.polyglot.t('orderDetail.summaryTab.fulfilled.yourNoteLabel'),
          });
        } else {
          this.vendor.getProfile()
            .done((profile) => {
              this.$refs.fulfilled && this.$refs.fulfilled.setState({
                noteFromLabel: app.polyglot.t('orderDetail.summaryTab.fulfilled.noteFromStoreLabel', { store: profile.get('name') }),
              });
            });
        }
      })

      if (this.shouldShowCompleteOrderForm()) this.showCompleteOrderForm = true;
    },

    renderOrderCompleteView () {
      const data = this.contract.get('orderComplete');
      if (!data) {
        throw new Error('Unable to create the Order Complete view because the buyerOrderCompletion '
          + 'data object has not been set.');
      }

      this.showCompleteOrderForm = false;

      this.showOrderComplete = true;

      this.$nextTick(() => {
        this.buyer.getProfile()
          .done((profile) => this.$refs.orderComplete && this.$refs.orderComplete.setState({ buyerName: profile.get('name') }));
      })
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

      this.disputeStartedOptions = {
        initialState: {
          ...data,
          showResolveButton: this.model.get('state') === 'DISPUTED'
            && this.model.isCase
            && (!paymentCoinData || !paymentCoinData.supportsEscrowTimeout),
        },
      };
      this.showDisputeStarted = true;

      this.$nextTick(() => {
        // this is only set on the Case.
        const buyerOpened = this.model.get('buyerOpened');
        if (typeof buyerOpened !== 'undefined') {
          const disputeOpener = buyerOpened ? this.buyer : this.vendor;
          disputeOpener.getProfile()
            .done((profile) => this.$refs.disputeStarted && this.$refs.disputeStarted.setState({ disputerName: profile.get('name') }));
        }
      })
    },

    renderDisputePayoutView () {
      const data = this.model.isCase ? this.model.get('disputeClose')
        : this.contract.get('disputeClose');
      if (!data) {
        throw new Error('Unable to create the Dispute Payout view because the resolution '
          + 'data object has not been set.');
      }

      this.showDisputePayout = true;

      this.$nextTick(() => {
        ['buyer', 'vendor', 'moderator'].forEach((type) => {
          this[type].getProfile().done((profile) => {
            const state = {};
            state[`${type}Name`] = profile.get('name');
            state[`${type}AvatarHashes`] = profile.get('avatarHashes').toJSON();

            if (this.$refs.disputePayout) {
              this.$refs.disputePayout.setState(state);
            }
          });
        });
      });
    },

    renderPayForOrder () {
      const { paymentCoin } = this.model;

      if (getWalletCurByCode(paymentCoin)) {
        this.showPayForOrder = true;
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

      this.disputeAcceptanceOptions = {
        initialState: {
          timestamp: data.timestamp,
          acceptedByBuyer: closer.id === this.buyer.id,
          buyerViewing: app.profile.id === this.buyer.id,
          vendorProcessingError: this.model.vendorProcessingError,
        },
      };
      this.showDisputeAcceptance = true;

      this.$nextTick(() => {
        closer.getProfile()
          .done((profile) => this.$refs.disputeAcceptance && this.$refs.disputeAcceptance.setState({
            closerName: profile.get('name'),
            closerAvatarHashes: profile.get('avatarHashes').toJSON(),
          }));
      });

      if (this.shouldShowCompleteOrderForm()) this.showCompleteOrderForm = true;
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
        this.ProcessingError = false;

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

      this.processingErrorOptions = {
        orderID: this.model.id,
        initialState: state,
      };

      if (this.ProcessingError) {
        this.$refs.processingError.setState(state);
      } else {
        this.ProcessingError = true;
      }
    },

    remove () {
      clearTimeout(this.disputeCountdownTimeout);
    },

    render () {
      if (this.shouldShowPayForOrderSection()) {
        this.renderPayForOrder();
      }

      this.renderTimeoutInfoView();

      if (this.shouldShowAcceptedSection()) this.renderAcceptedView();
      this.renderSubSections();
      this.renderProcessingError();

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
