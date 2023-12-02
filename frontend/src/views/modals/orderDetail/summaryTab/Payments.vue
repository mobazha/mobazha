<template>
  <div class="payments">
    <template v-for="(payment, index) in reversedPayments">
      <Payment ref="payments" :options="paymentOptions(payment, reversedPayments.length - index)"
        :bb="() => {
          return {
            model: payment,
          }
        }"
        @cancelClick="onCancelClick"
        @acceptClick="onAcceptClick"
        @confirmedRejectClick="onRejectClick"
      />
    </template>
  </div>
</template>
  
<script>
import app from '../../../../../backbone/app.js';
import bigNumber from 'bignumber.js';
import {
  acceptingOrder,
  acceptOrder,
  rejectingOrder,
  rejectOrder,
  cancelingOrder,
  cancelOrder,
  events as orderEvents,
} from '../../../../../backbone/utils/order.js';
import { isValidCoinDivisibility } from '../../../../../backbone/utils/currency';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../../backbone/data/walletCurrencies.js';
import { checkValidParticipantObject } from '../../../../utils/utils';

import Payment from './Payment.vue';

export default {
  components: {
    Payment,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      collectionKey: 0,

      vendorName: '',

      _options: {},
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
      return {
        ...this.templateHelpers,
        options: {
          isCrypto: false,
          ...this.options,
        }
      };
    },
    paymentCoinData() {
      let paymentCoinData = {};
      try {
        paymentCoinData = getWalletCurByCode(this.options.paymentCoin);
      } catch (e) {
        // pass
      }
      return paymentCoinData;
    },
    paidSoFar() {
      let access = this.collectionKey;

      let paidResult = [];
      this.collection.models.forEach((payment, index) => {
        let paidSoFar = this.collection.models
          .slice(0, index + 1)
          .reduce((total, model) => total.plus(model.get('value')), bigNumber('0'));

        if (isValidCoinDivisibility[0]) {
          // round based on divisibility
          paidSoFar = paidSoFar.dp(this.paymentCoinData.coinDivisibility);
        }
        paidResult.push(paidSoFar);
      });

      return paidResult;
    },
    reversedPayments() {
      let access = this.collectionKey;

      return this.collection.models.slice().reverse();
    }
  },
  methods: {
    loadData (options) {
      const opts = {
        isCrypto: false,
        ...options,
      };

      if (!options.orderID) {
        throw new Error('Please provide the order id.');
      }

      if (!options.collection) {
        throw new Error('Please provide a transactions collection.');
      }

      if (!(options.orderPrice instanceof bigNumber)) {
        throw new Error('Please provide the price of the order as a BigNumber '
          + 'instance.');
      }

      if (typeof options.isOrderCancelable !== 'function') {
        throw new Error('Please provide a function that returns whether this order can be canceled '
          + 'by the current user.');
      }

      if (typeof options.isOrderConfirmable !== 'function') {
        throw new Error('Please provide a function that returns whether this order can be '
          + 'confirmed by the current user.');
      }

      checkValidParticipantObject(options.vendor, 'vendor');

      this.baseInit(opts);
      this.__options = opts;

      options.vendor.getProfile()
        .done((profile) => {
          this.vendorName = profile.get('name') || '';

          this.collectionKey += 1;
        });

      this.listenTo(this.collection, 'update', () => this.collectionKey += 1);

      this.listenTo(orderEvents, 'cancelingOrder', this.onCancelingOrder);
      this.listenTo(orderEvents, 'cancelOrderComplete, cancelOrderFail', this.onCancelOrderAlways);
      this.listenTo(orderEvents, 'cancelOrderComplete', this.onAcceptOrderComplete);
      this.listenTo(orderEvents, 'acceptingOrder', this.onAcceptingOrder);
      this.listenTo(orderEvents, 'acceptOrderComplete acceptOrderFail', this.onAcceptOrderAlways);
      this.listenTo(orderEvents, 'acceptOrderComplete', this.onAcceptOrderComplete);
      this.listenTo(orderEvents, 'rejectingOrder', this.onRejectingOrder);
      this.listenTo(orderEvents, 'rejectOrderComplete rejectOrderFail', this.onRejectOrderAlways);
      this.listenTo(orderEvents, 'rejectOrderComplete', this.onRejectOrderComplete);
    },

    setLastPaymentState(state) {
      if (this.$refs.payments) {
        this.$refs.payments[0].setState(state);
      }
    },

    onCancelClick() {
      cancelOrder(this.orderID);
    },

    onCancelingOrder(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ cancelInProgress: true });
      }
    },

    onCancelOrderAlways(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ cancelInProgress: false });
      }
    },

    onCancelOrderComplete(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ showCancelButton: false });
      }
    },

    onAcceptClick() {
      acceptOrder(this.orderID);
    },

    onAcceptingOrder(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ acceptInProgress: true });
      }
    },

    onAcceptOrderAlways(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ acceptInProgress: false });
      }
    },

    onAcceptOrderComplete(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ showAcceptButton: false });
      }
    },

    onRejectClick() {
      rejectOrder(this.orderID);
    },

    onRejectingOrder(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ rejectInProgress: true });
      }
    },

    onRejectOrderAlways(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ rejectInProgress: false });
      }
    },

    onRejectOrderComplete(e) {
      if (e.id === this.orderID) {
        setLastPaymentState({ showRejectButton: false });
      }
    },

    isMostRecentPayment(index) {
      return index === this.collection.length - 1;
    },

    blockChainTxUrl(paymentId) {
      let blockChainTxUrl = '';

      try {
        blockChainTxUrl = this.paymentCoinData.getBlockChainTxUrl(
          paymentId,
          app.serverConfig.testnet,
        );
      } catch (e) {
        // pass
      }

      return blockChainTxUrl;
    },

    paymentOptions(payment, index) {
      return {
        initialState: {
          paymentNumber: index,
          amountShort: this.options.orderPrice.minus(this.paidSoFar[index]),
          showAcceptRejectButtons: this.isMostRecentPayment(index) && this.options.isOrderConfirmable(),
          showCancelButton: this.isMostRecentPayment(index) && this.options.isOrderCancelable(),
          cancelInProgress: cancelingOrder(this.orderID),
          acceptInProgress: acceptingOrder(this.orderID),
          rejectInProgress: rejectingOrder(this.orderID),
          isCrypto: this._options.isCrypto,
          blockChainTxUrl: this.blockChainTxUrl(payment.id),
          paymentCoin: this.paymentCoinData.code,
          paymentCoinDivis: this.paymentCoinData.coinDivisibility,

          payee: this.vendorName,
        },
      };
    },
  }
}
</script>
<style lang="scss" scoped></style>
