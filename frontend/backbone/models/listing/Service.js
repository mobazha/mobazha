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
      price: '',
      additionalWeightPrice: '',
      estimatedDelivery: '',
      currency: '',
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
        amount: attrs.price,
        currency: curDefCurrency,
      },
      addError,
      `price`,
      {
        validationOptions: {
          rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
        },
      }
    );

    this.validateCurrencyAmount(
      {
        amount: attrs.additionalWeightPrice,
        currency: curDefCurrency,
      },
      addError,
      `additionalWeightPrice`,
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
