/* eslint-disable class-methods-use-this */
import bigNumber from 'bignumber.js';
import app from '../../app';
import BaseOrder from './BaseOrder';
import Contract from './Contract';
import Transactions from '../../collections/order/Transactions';

class Order extends BaseOrder {
  constructor(attrs, options) {
    const opts = {
      type: 'sale',
      ...options,
    };

    const types = ['sale', 'purchase'];

    if (types.indexOf(opts.type) === -1) {
      throw new Error(`Type needs to be one of ${types}.`);
    }

    super(attrs, opts);
    this.type = opts.type;
  }

  url() {
    return app.getServerUrl(`ob/order/${this.id}`);
  }

  get idAttribute() {
    return 'orderID';
  }

  get nested() {
    return {
      contract: Contract,
    };
  }

  /**
   * Returns a boolean indicating whether the vendor had an error when processing
   * the order. This is different from just checking for the PROCESSING_ERROR state,
   * in that it will return true even after the order moves on from that state.
   */
  get vendorProcessingError() {
    const contract = this.get('contract');
    return contract && Array.isArray(contract.get('erroredMessages'));
  }

  get orderPrice() {
    let orderPrice;

    try {
      orderPrice = this.contract
        .get('orderOpen')
        .payment
        .amount;
    } catch (e) {
      // pass
    }

    return (
      orderPrice instanceof bigNumber
        ? orderPrice : bigNumber()
    );
  }

  get totalPaid() {
    return this.paymentsIn
      .reduce((total, transaction) => total.plus(transaction.value), bigNumber(0));
  }

  getBalanceRemaining() {
    return this.orderPrice.minus(this.totalPaid);
  }

  get isPartiallyFunded() {
    return !this.get('funded');
  }

  /**
   * Returns a boolean indicating whether the order has been partially or fully
   * funded.
   */
  get isFunded() {
    return this.get('funded');
  }

  /**
   * Returns the block height in which this order became fully funded. If the order is
   * not fully funded or the transaction(s) that would make it fully funded haven't
   * confirmed, it will return 0.
   */
  get fundedBlockHeight() {
    const height = 0;

    // const models = this.paymentsIn
    //   .filter(payment => {
    //     const paymentHeight = payment.get('height');
    //     return typeof paymentHeight === 'number' && paymentHeight > 0;
    //   })
    //   .sort((a, b) => (a.get('height') - b.get('height')));

    // _.every(models, (payment, pIndex) => {
    //   const transactions = new Transactions(
    //     models.slice(0, pIndex + 1),
    //     { paymentCoin: this.paymentCoin }
    //   );
    //   if (this.getBalanceRemaining(transactions) <= 0) {
    //     height = payment.get('height');
    //     return false;
    //   }

    //   return true;
    // });

    return height;
  }

  /**
   * Returns a modified version of the transactions by filtering out any negative payments
   * (e.g. money moving from the multisig to the vendor, refunds).
   */
  get paymentsIn() {
    return new Transactions(this.get('contract').get('transactions'));
  }

  get isOrderCancelable() {
    return this.buyerID === app.profile.id
      && !this.moderatorID
      && ['PROCESSING_ERROR', 'PENDING'].includes(this.get('state'))
      && this.isFunded;
  }

  get isOrderDisputable() {
    const orderState = this.get('state');

    if (this.buyerID === app.profile.id) {
      return !!this.moderatorID
        && (
          ['AWAITING_FULFILLMENT', 'PENDING', 'FULFILLED'].includes(orderState)
          || (orderState === 'PROCESSING_ERROR' && this.isFunded)
        );
    }

    if (this.vendorID === app.profile.id) {
      return !!this.moderatorID
        && ['PARTIALLY_FULFILLED', 'FULFILLED'].includes(orderState);
    }

    return false;
  }

  parse(response = {}) {
    this.rawResponse = JSON.parse(JSON.stringify(response)); // deep clone;

    if (response.contract) {
      // Since we modify the data on parse (particularly in some nested models),
      // we'll store the original contract here.
      response.rawContract = this.rawResponse.contract;

      response.contract = Order.parseContract(response.contract);

      response.contract.disputeClose = Order.parseDisputePayout(
        response.contract.disputeClose,
        response.contract.orderOpen?.payment?.coin,
      );
    }

    return response;
  }
}

export default Order;
