import { curDefToDecimal } from '../../utils/currency';
import BaseModel from '../BaseModel';

export default class extends BaseModel {
  get idAttribute() {
    return 'txid';
  }

  parse(response = {}) {
    console.log('temp pending ob-go/#1803');
    if (response.value.startsWith('--')) {
      response.value = response.value.slice(1);
    }

    return {
      ...response,
      value: curDefToDecimal({
        amount: response.value,
        currency: response.currency,
      }),
    };
  }
}
