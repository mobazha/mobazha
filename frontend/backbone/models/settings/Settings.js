/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import app from '../../app';
import BaseModel from '../BaseModel';
import ShippingAddresses from '../../collections/ShippingAddresses';
import ShippingOptions from '../../collections/settings/ShippingOptions'
import SMTPSettings from './SMTPSettings';

import {
  decimalToInteger,
  integerToDecimal,
  isValidCoinDivisibility,
  getCoinDivisibility,
} from '../../utils/currency';

export default class extends BaseModel {
  defaults() {
    return {
      paymentDataInQR: false,
      showNotifications: true,
      showNsfw: false,
      localCurrency: 'USD',
      country: 'UNITED_STATES',
      termsAndConditions: '',
      refundPolicy: '',
      blockedNodes: [],
      storeModerators: [],
      externalPaymentAddresses: {},
      shippingAddresses: new ShippingAddresses(),
      shippingOptions: new ShippingOptions(),
      smtpSettings: new SMTPSettings(),
    };
  }

  url() {
    return app.getServerUrl('ob/preferences');
  }

  nested() {
    return {
      shippingAddresses: ShippingAddresses,
      shippingOptions: ShippingOptions,
      smtpSettings: SMTPSettings,
    };
  }

  ownMod(guid) {
    if (!guid) {
      throw new Error('Please provide a guid.');
    }

    return this.get('storeModerators').indexOf(guid) !== -1;
  }

  get prettyServerVer() {
    const sVer = this.get('userAgent');
    return sVer.substring(sVer.lastIndexOf(':') + 1, sVer.lastIndexOf('/'));
  }

  validate(attrs) {
    const errObj = this.mergeInNestedErrors({});

    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    if (!_.isArray(attrs.storeModerators)) {
      // this error should never be visible to the user
      addError('storeModerators', 'The storeModerators is invalid because it is not an array');
    }

    if (!(attrs.shippingOptions instanceof ShippingOptions)) {
      addError('shippingOptions', 'A nested ShippingOptions collection is required.');
    }

    // check no duplicate shippingOption name
    const optionNames = attrs.shippingOptions.map((option) => option.get('name'));
    if (_.uniq(optionNames).length != optionNames.length) {
      addError('shippingOptions', 'ShippingOptions names cannot be duplicated.');
    }

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }

  sync(method, model, options) {
    if (method === 'create' && typeof options.type === 'undefined') {
      // we will use PUT unless you explicitly save with POST,
      // e.g. model.save({}, { type: 'POST' })
      options.type = 'PUT';
    }

    if (method !== 'read' && method !== 'delete' && options.attrs) {
      (options.attrs.shippingOptions ?? []).forEach(shipOpt => {
        if (shipOpt.services && shipOpt.services.length) {
          const coinDiv = getCoinDivisibility(shipOpt.currency);

          shipOpt.services.forEach(service => {
            service.firstFreight = decimalToInteger(
              service.firstFreight,
              coinDiv
            );
            service.renewalUnitPrice = decimalToInteger(
              service.renewalUnitPrice,
              coinDiv
            );
            service.registrationFee = decimalToInteger(
              service.registrationFee,
              coinDiv
            );
          });
        }
      });
    }

    return super.sync(method, model, options);
  }

  parse(response = {}) {
    if (Array.isArray(response.blockedNodes)) {
      // de-dupe
      response.blockedNodes = Array.from(new Set(response.blockedNodes));

      // do not allow own node to be in the blocked list
      response.blockedNodes = response.blockedNodes
        .filter((peerID) => peerID !== app.profile.id);
    }

    if (response.shippingOptions && response.shippingOptions.length) {
      response.shippingOptions.forEach((shipOpt, shipOptIndex) => {
        const currencyCode = shipOpt.currency;
        let coinDiv;
        try {
          coinDiv = getCoinDivisibility(currencyCode);
        } catch (e) {
          // pass
        }

        const [isValidCoinDiv] = isValidCoinDivisibility(coinDiv);

        if (!isValidCoinDiv) {
          console.error('Unable to convert price fields. The coin divisibility is not valid. Currency: ', currencyCode);
        }

        if (shipOpt.services && shipOpt.services.length) {
          shipOpt.services.forEach(service => {
            service.firstFreight = integerToDecimal(
              service.firstFreight,
              coinDiv,
              { fieldName: 'service.firstFreight' }
            );
            service.renewalUnitPrice = integerToDecimal(
              service.renewalUnitPrice,
              coinDiv,
              { fieldName: 'service.renewalUnitPrice' }
            );
            service.registrationFee = integerToDecimal(
              service.registrationFee,
              coinDiv,
              { fieldName: 'service.registrationFee' }
            );
          });
        }
      });
    }

    return response;
  }
}
