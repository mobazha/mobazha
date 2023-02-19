import bigNumber from 'bignumber.js';
import { getCurrencyByCode as getWalletCurByCode } from '../../data/walletCurrencies';
import {
  curDefToDecimal,
  integerToDecimal,
  getCoinDivisibility,
  defaultCryptoCoinDivisibility,
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
    return typeof attrs.disputeOpen !== 'undefined';
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
      contract = attrs.disputeOpen.openedBy === 'BUYER' ?
        attrs.buyerContract :
        attrs.vendorContract;
    }

    return contract;
  }

  get contract() {
    let contract = this.get('contract');

    if (this.isCase) {
      contract = this.get('disputeOpen').openedBy == "BUYER" ?
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
    return attrs.completable;
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
        let coinDiv;
        try {
          coinDiv = getCoinDivisibility(payment.coin);
        } catch (e) {
          coinDiv = defaultCryptoCoinDivisibility;
        }

        payment.amount = curDefToDecimal({
          amount: payment.amount,
          currency: {
            code: payment.coin,
            divisibility: coinDiv,
          }
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

  static parseDisputePayout(disputeClose, coin) {
    let divisibility;
    try {
      divisibility = getCoinDivisibility(coin);
    } catch (e) {
      divisibility = defaultCryptoCoinDivisibility;
    }

    if (disputeClose && disputeClose.releaseInfo) {
      if (disputeClose.releaseInfo.buyerAmount) {
        // legacy check
        if (disputeClose.releaseInfo.buyerAmount === '') {
          disputeClose.releaseInfo.buyerAmount = integerToDecimal(
            disputeClose.releaseInfo.buyerAmount,
            8
          );
        } else {
          disputeClose.releaseInfo.buyerAmount =
            integerToDecimal(
              disputeClose.releaseInfo.buyerAmount,
              divisibility,
              { fieldName: 'releaseInfo.buyerAmount' }
            );
        }
      }

      if (disputeClose.releaseInfo.vendorAmount) {
        // legacy check
        if (disputeClose.releaseInfo.vendorAmount === '') {
          disputeClose.releaseInfo.vendorAmount = integerToDecimal(
            disputeClose.releaseInfo.vendorAmount,
            8
          );
        } else {
          disputeClose.releaseInfo.vendorAmount =
            integerToDecimal(
              disputeClose.releaseInfo.vendorAmount,
              divisibility,
              { fieldName: 'releaseInfo.vendorAmount' }
            );
        }
      }

      if (disputeClose.releaseInfo.moderatorAmount) {
        // legacy check
        if (disputeClose.releaseInfo.moderatorAmount === '') {
          disputeClose.releaseInfo.moderatorAmount = integerToDecimal(
            disputeClose.releaseInfo.moderatorAmount,
            8
          );
        } else {
          disputeClose.releaseInfo.moderatorAmount =
            integerToDecimal(
              disputeClose.releaseInfo.moderatorAmount,
              divisibility,
              { fieldName: 'releaseInfo.moderatorAmount' }
            );
        }
      }
    }

    return disputeClose;
  }
}
