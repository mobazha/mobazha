import BaseOrder from './BaseOrder';
import Contract from './Contract';
import app from '../../app';

class Case extends BaseOrder {
  url() {
    return app.getServerUrl(`ob/case/${this.id}`);
  }

  get idAttribute() {
    return 'caseID';
  }

  get nested() {
    return {
      vendorContract: Contract,
      buyerContract: Contract,
    };
  }

  /**
   * Returns a boolean indicating whether the vendor had an error when processing
   * the order. Since this relies on data from the buyers contract, if the contract is
   * not verified as authentic, false will be returned even if the data suggests
   * otherwise. We don't want to prevent funds from being awared to the vendor based
   * on potentially forged data from the buyer's contract.
   */
  get vendorProcessingError() {
    const contract = this.get('buyerContract');
    const contractErrors = this.get('buyerContractValidationErrors');
    return !!contract &&
      Array.isArray(contract.get('errors')) &&
      (
        !contractErrors ||
        !(Array.isArray(contractErrors) && contractErrors.length)
      );
  }

  /**
   * - If the contract hasn't arrived, using this logic, it will be considered invalid.
   * - If the vendor had an error processing the order and the buyer's contract is verified
   *   as authentic, the vendor's contract will be considered valid even though it will not
   *   be sent over, since the mod will only be allowed to send the funds to the buyer.
   */
  isContractValid(buyer = true) {
    const hasContractArrived = buyer ?
      !!this.get('buyerContract') :
      !!this.get('vendorContract');
    const errors = buyer ?
      this.get('buyerContractValidationErrors') :
      this.get('vendorContractValidationErrors');

    return hasContractArrived &&
      (!errors ||
        (Array.isArray(errors) && !errors.length)) ||
      !buyer && this.vendorProcessingError;
  }

  get isBuyerContractValid() {
    return this.isContractValid();
  }

  get isVendorContractValid() {
    return this.isContractValid(false);
  }

  get bothContractsValid() {
    return this.isBuyerContractValid && this.isVendorContractValid;
  }

  get isOrderCancelable() {
    return false;
  }

  get isOrderDisputable() {
    return false;
  }

  parse(response = {}) {
    // If only one contract has arrived, we'll fire an event when the other one comes
    if (!this._otherContractEventBound &&
      !this.vendorProcessingError &&
      (
        (response.buyerOpened && !response.vendorContract) ||
        (!response.buyerOpened && !response.buyerContract)
      )
    ) {
      const needBuyer = !response.buyerContract;
      this._otherContractEventBound = true;
      this.once(`change:${needBuyer ? 'buyer' : 'vendor'}Contract`,
        () => this.trigger('otherContractArrived', this, { isBuyer: needBuyer }));
    }

    if (response.buyerContract) {
      // Since we modify the data on parse (particularly in some nested models),
      // we'll store the original contract here.
      response.rawBuyerContract =
        JSON.parse(JSON.stringify(response.buyerContract)); // deep clone

      response.buyerContract = Case.parseContract(response.buyerContract);
    }

    if (response.vendorContract) {
      // Since we modify the data on parse (particularly in some nested models),
      // we'll store the original contract here.
      response.rawVendorContract =
        JSON.parse(JSON.stringify(response.vendorContract)); // deep clone

      response.vendorContract = Case.parseContract(response.vendorContract);
    }

    response.disputeClose =
      Case.parseDisputePayout(response.disputeClose, this.paymentCoin);

    return response;
  }
}

export default Case;
