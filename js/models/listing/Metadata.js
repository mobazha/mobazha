import app from '../../app';
import BaseModel from '../BaseModel';
import is from 'is_js';
import { isSupportedWalletCur } from '../../data/walletCurrencies';
import { getCurrencyByCode } from '../../data/currencies';
import { isValidCoinDivisibility } from '../../utils/currency';

export default class extends BaseModel {
  defaults() {
    return {
      contractType: 'PHYSICAL_GOOD',
      format: 'FIXED_PRICE', // this is not in the design at this time
      // by default, setting to "never" expire (due to a unix bug, the max is before 2038)
      expiry: (new Date(2037, 11, 31, 0, 0, 0, 0)).toISOString(),
      acceptedCurrencies: [
        ...(app && app.profile && app.profile.get('currencies') || []),
      ],
    };
  }

  get contractTypes() {
    return [
      'PHYSICAL_GOOD',
      'DIGITAL_GOOD',
      'SERVICE',
      'CRYPTOCURRENCY',
    ];
  }

  get contractTypesVerbose() {
    return this.contractTypes
      .map((contractType) => (
        {
          code: contractType,
          name: app.polyglot.t(`formats.${contractType}`),
        }
      ));
  }

  get formats() {
    return [
      'FIXED_PRICE',
      'MARKET_PRICE',
    ];
  }

  set(key, val, options = {}) {
    // Handle both `"key", value` and `{key: value}` -style arguments.
    let attrs;
    let opts = options;

    if (typeof key === 'object') {
      attrs = key;
      opts = val || {};
    } else {
      (attrs = {})[key] = val;
    }

    return super.set(attrs, opts);
  }

  validate(attrs) {
    const errObj = {};
    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    if (!this.contractTypes.includes(attrs.contractType)) {
      addError('contractType', `The contract type must be one of ${this.contractTypes}.`);
    }

    if (!this.formats.includes(attrs.format)) {
      addError('format', `The format must be one of ${this.formats}.`);
    }

    const firstDayOf2038 = new Date(2038, 0, 1, 0, 0, 0, 0);

    // please provide data as ISO string (or possibly unix timestamp)
    // todo: validate date is provided in the the right format
    if (is.not.inDateRange(new Date(attrs.expiry), new Date(Date.now()), firstDayOf2038)) {
      addError('expiry', 'The expiration date must be between now and the year 2038.');
    }

    if (!Array.isArray(attrs.acceptedCurrencies) || !attrs.acceptedCurrencies.length) {
      const translationKey = attrs.contractType === 'CRYPTOCURRENCY' ?
        'metadataModelErrors.provideAcceptedCurrencyCrypto' :
        'metadataModelErrors.provideAcceptedCurrency';
      addError('acceptedCurrencies',
        app.polyglot.t(translationKey));
    } else if (attrs.acceptedCurrencies.findIndex(cur => (typeof cur !== 'string' || !cur)) !==
      -1) {
      // Ensure only non-empty strings are provided as accepted currencies
      addError('acceptedCurrencies', 'Accepted currency values must be non-empty strings.');
    } else {
      // Ensure only supported wallet currencies are provided as accepted currencies
      const unsupportedCurrencies = attrs.acceptedCurrencies
        .filter(cur => !isSupportedWalletCur(cur));

      if (unsupportedCurrencies.length) {
        addError('acceptedCurrencies', app.polyglot.t('metadataModelErrors.unsupportedAcceptedCurs',
          { curs: unsupportedCurrencies.join(', ') }));
      }
    }

    if (attrs.contractType === 'CRYPTOCURRENCY') {
      if (Array.isArray(attrs.acceptedCurrencies) && attrs.acceptedCurrencies.length > 1) {
        addError('acceptedCurrencies', 'For cryptocurrency listings, only one acccepted ' +
          'currency is allowed.');
      }

      if (!attrs.cryptoListingCurrencyCode || typeof attrs.cryptoListingCurrencyCode !== 'string') {
        addError('cryptoListingCurrencyCode', 'Please provide a cryptoListingCurrencyCode.');
      }
    } else {
      // The ones in this block should not be user facing unless there's a dev error.
      if (typeof attrs.pricingCurrency !== 'object') {
        addError('pricingCurrency', 'The pricingCurrency must be provided as an object.');
      } else {
        if (
          !attrs.pricingCurrency.code ||
          !getCurrencyByCode(attrs.pricingCurrency.code)
        ) {
          addError('pricingCurrency.code', 'The currency is not one of the available ones.');
        }

        if (!isValidCoinDivisibility(attrs.pricingCurrency.divisibility)[0]) {
          addError('pricingCurrency.divisibility', 'The divisibility is not valid.');
        }
      }
    }

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }
}
