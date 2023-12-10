import app from '../../app';
import is from 'is_js';
import BaseModel from '../BaseModel';
import {
  getCoinDivisibility,
  CUR_VAL_RANGE_TYPES,
} from '../../utils/currency';

export default class extends BaseModel {
  defaults() {
    return {
      name: '',
      estimatedDelivery: '',
      currency: '',

      startWeight: 0,
      endWeight: 0,
      firstWeight: 0,
      firstFreight: '',
      renewalUnitWeight: 0,
      renewalUnitPrice: '',
      registrationFee: '',
    };
  }

  get idAttribute() {
    return '_clientID';
  }

  validate(attrs) {
    const errObj = {};
    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    if (is.not.string(attrs.name)) {
      addError('name', 'Please provide a name as a string.');
    } else if (!attrs.name) {
      addError('name', app.polyglot.t('serviceModelErrors.provideName'));
    }

    if (is.not.string(attrs.estimatedDelivery)) {
      addError('estimatedDelivery', 'Please provide an estimated delivery time as a string.');
    } else if (!attrs.estimatedDelivery) {
      addError('estimatedDelivery', app.polyglot.t('serviceModelErrors.provideEstDeliveryTime'));
    }

    const curDefCurrency = {
      code: () => attrs.currency,
      divisibility: () => getCoinDivisibility(attrs.currency),
    };
    
    this.validateCurrencyAmount(
      {
        amount: attrs.firstFreight,
        currency: curDefCurrency,
      },
      addError,
      `firstFreight`,
      {
        validationOptions: {
          rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
        },
      }
    );

    this.validateCurrencyAmount(
      {
        amount: attrs.renewalUnitPrice,
        currency: curDefCurrency,
      },
      addError,
      `renewalUnitPrice`,
      {
        validationOptions: {
          rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
        },
      }
    );

    this.validateCurrencyAmount(
      {
        amount: attrs.registrationFee,
        currency: curDefCurrency,
      },
      addError,
      `registrationFee`,
      {
        validationOptions: {
          rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
        },
      }
    );

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }
}
