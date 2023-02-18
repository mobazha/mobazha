import BaseModel from '../BaseModel';
import app from '../../app';
import { getCurrencyByCode } from '../../data/currencies';
import { getCoinDivisibility } from '../../utils/currency';

export default class extends BaseModel {
  defaults() {
    return {
      currency: {code: 'USD'},
    };
  }

  validate(attrs) {
    const errObj = {};
    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    let coinDiv;

    if (
      typeof attrs.currency.code !== 'string' ||
      !attrs.currency.code ||
      !getCurrencyByCode(attrs.currency.code)
    ) {
      addError('currencyCode', app.polyglot.t('fixedFeeModelErrors.noCurrency'));
    } else {
      try {
        coinDiv = getCoinDivisibility(attrs.currency.code);
      } catch (e) {
        // pass
      }

      this.validateCurrencyAmount(
        {
          amount: attrs.amount,
          currency: {
            code: attrs.currency.code,
            divisibility: coinDiv,
          },
        },
        addError,
        'amount',
        {
          translations: {
            range: 'fixedFeeModelErrors.amountGreaterThanZero',
            type: 'fixedFeeModelErrors.fixedFeeAsNumber',
            required: 'fixedFeeModelErrors.fixedFeeAsNumber',
          },
        }
      );
    }

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }
}
