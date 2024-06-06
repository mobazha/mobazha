import is from 'is_js';
import bigNumber from 'bignumber.js';
import BaseModel from '../BaseModel';
import Image from './Image';

export default class extends BaseModel {
  defaults() {
    return {
      featureID: '',
      name: '',
      surcharge: bigNumber('0'),
      skuID: '',
    };
  }

  get nested() {
    return {
      image: Image,
    };
  }

  get idAttribute() {
    return 'featureID';
  }

  static get maxFilenameLength() {
    return 255;
  }

  get max() {
    return {
      featureIDLength: 40,
    };
  }

  validate(attrs) {
    const errObj = {};
    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    if (attrs.featureID.length > this.max.featureIDLength) {
      addError('featureID', `The featureID cannot exceed ${this.max.featureIDLength} characters.`);
    }

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
