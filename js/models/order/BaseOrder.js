import bigNumber from 'bignumber.js';
import { getCurrencyByCode as getWalletCurByCode } from '../../data/walletCurrencies';
import {
  curDefToDecimal,
  integerToDecimal,
} from '../../utils/currency';
import BaseModel from '../BaseModel';

export default class extends BaseModel {
  // Many methods below are exposed both as static and instance getters, with the
  // latter being a proxy to the former. The reason for them being exposed as static
  // is because the instance variety requires all necessary data to be on the model
  // and there are cases where that won't be the case (e.g. in parse), yet the data
  // is available to be passed in. The instance getters are there as a convenience
  // (slightly less syntax to call) and in most cases the model will be synced, so they
  // are the option that is used.

  // PLEASE NOTE: The majority of the functions below will only return an
  // accurate value if the attribute set of the model is passed in after
  // it was obtained from the server. In other words, if only partial data
  // where provided, it may lead to inaccurate results (or exceptions).

  static isCase(attrs = {}) {
    return typeof attrs.buyerOpened !== 'undefined';
  }

  get isCase() {
    return this.constructor.isCase(this.toJSON());
  }

  /**
   * Returns the contract. If this is a case, it will return the contract of the
   * party that opened the dispute, which is the only contract you're guaranteed
   * to have. If you need the specific contract of either the buyer or seller,
   * grab it directly via model.get('buyerContract') / model.get('vendorContract').
   */
  static getContract(attrs = {}) {
    let contract = attrs.contract;

    if (this.isCase(attrs)) {
      contract = attrs.buyerOpened ?
        attrs.buyerContract :
        attrs.vendorContract;
    }

    return contract;
  }

  get contract() {
    let contract = this.get('contract');

    if (this.isCase) {
      contract = this.get('buyerOpened') ?
        this.get('buyerContract') :
        this.get('vendorContract');
    }

    return contract;
  }

  static getParticipantIDs(attrs = {}) {
    return {
      buyer: this.getContract(attrs).orderOpen.buyerID.peerID,
      vendor: this.getContract(attrs).orderOpen.listings[0].listing.vendorID.peerID,
      moderator: this.getContract(attrs).orderOpen.payment.moderator,
    };
  }

  get participantIDs() {
    return this.constructor.getParticipantIDs(this.toJSON());
  }

  static getBuyerID(attrs = {}) {
    return this.getParticipantIDs(attrs).buyer;
  }

  get buyerID() {
    return this.constructor.getBuyerID(this.toJSON());
  }

  static getVendorID(attrs = {}) {
    return this.getParticipantIDs(attrs).vendor;
  }

  get vendorID() {
    return this.constructor.getVendorID(this.toJSON());
  }

  static getModeratorID(attrs = {}) {
    return this.getParticipantIDs(attrs).moderator;
  }

  get moderatorID() {
    return this.constructor.getModeratorID(this.toJSON());
  }

  static canBuyerComplete(attrs = {}) {
    return attrs.canComplete;
  }

  get canBuyerComplete() {
    return this.constructor.canBuyerComplete(this.toJSON());
  }

  static getPaymentCoin(attrs = {}) {
    let paymentCoin = '';

    try {
      paymentCoin =
        this.getContract(attrs)
          .orderOpen
          .payment
          .coin;
    } catch (e) {
      // pass
    }

    return paymentCoin;
  }


  get paymentCoin() {
    return this.constructor.getPaymentCoin(this.toJSON());
  }

  static getPaymentCoinData(attrs = {}) {
    let curData;

    try {
      curData = getWalletCurByCode(this.getPaymentCoin(attrs));
    } catch (e) {
      // pass
    }

    return curData;
  }

  get paymentCoinData() {
    return this.constructor.getPaymentCoinData(this.toJSON());
  }

  static parseContract(contract) {
    if (contract) {
      let payment;

      try {
        payment = contract.orderOpen.payment;
      } catch (e) {
        // pass
      }

      if (payment) {
        payment.amount = curDefToDecimal({
          amount: payment.amount,
          currency: payment.coin,
        });
      }

      // convert crypto listing quantities
      contract.orderOpen.items.forEach((item, index) => {
        try {
          const listing = contract.orderOpen.listings[index].listing;

          if (listing.metadata.contractType === 'CRYPTOCURRENCY') {
            const divisibility = listing
              .metadata
              .pricingCurrency
              .divisibility;

            item.quantity = integerToDecimal(item.quantity, divisibility);
          }
        } catch (e) {
          item.quantity = bigNumber();
        }
      });
    }

    return contract;
  }

  static parseDisputePayout(resolution) {
    let divisibility;

    try {
      divisibility =
        resolution
          .payout
          .payoutCurrency
          .divisibility;
    } catch (e) {
      // pass
    }

    if (resolution && resolution.payout) {
      if (resolution.payout.buyerOutput) {
        // legacy check
        if (resolution.payout.buyerOutput.amount === '') {
          resolution.payout.buyerOutput.amount = integerToDecimal(
            resolution.payout.buyerOutput.amount,
            8
          );
        } else {
          resolution.payout.buyerOutput.amount =
            integerToDecimal(
              resolution.payout.buyerOutput.amount,
              divisibility,
              { fieldName: 'buyerOutput.amount' }
            );
        }
      }

      if (resolution.payout.vendorOutput) {
        // legacy check
        if (resolution.payout.vendorOutput.amount === '') {
          resolution.payout.vendorOutput.amount = integerToDecimal(
            resolution.payout.vendorOutput.amount,
            8
          );
        } else {
          resolution.payout.vendorOutput.amount =
            integerToDecimal(
              resolution.payout.vendorOutput.amount,
              divisibility,
              { fieldName: 'vendorOutput.amount' }
            );
        }
      }

      if (resolution.payout.moderatorOutput) {
        if (resolution.payout.moderatorOutput) {
          // legacy check
          if (resolution.payout.moderatorOutput.amount === '') {
            resolution.payout.moderatorOutput.amount = integerToDecimal(
              resolution.payout.moderatorOutput.amount,
              8
            );
          } else {
            resolution.payout.moderatorOutput.amount =
              integerToDecimal(
                resolution.payout.moderatorOutput.amount,
                divisibility,
                { fieldName: 'moderatorOutput.amount' }
              );
          }
        }
      }
    }

    return resolution;
  }
}
