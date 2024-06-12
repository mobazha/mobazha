import is from 'is_js';
import bigNumber from 'bignumber.js';
import BaseModel from '../BaseModel';
import ListingImages from '../../collections/listing/ListingImages';

export default class extends BaseModel {
  defaults() {
    return {
      name: '',
      surcharge: bigNumber('0'),
      skuID: '',
      images: new ListingImages(),
    };
  }

  get nested() {
    return {
      images: ListingImages,
    };
  }

  get idAttribute() {
    return '_clientID';
  }

  static get maxFilenameLength() {
    return 255;
  }

  validate(attrs) {
    const errObj = {};
    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    if (!attrs.name) {
      addError('name', 'Please provide a feature name.');
    } else if (is.not.string(attrs.name)) {
      addError('name', 'Please provide a feature name as a string.');
    } else if (attrs.name.length > this.constructor.maxFilenameLength) {
      addError('name', `The name exceeds the max length of ${this.maxFilenameLength}`);
    }

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }
}
