import BaseModel from '../BaseModel';
import is from 'is_js';

export default class extends BaseModel {
  defaults() {
    return {
      downloadID: '',
      name: '',
      file: '',
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
      addError('name', 'Please provide a download name.');
    } else if (is.not.string(attrs.name)) {
      addError('name', 'Please provide a download name as a string.');
    } else if (attrs.name.length > this.constructor.maxFilenameLength) {
      addError('name', `The name exceeds the max length of ${this.maxFilenameLength}`);
    }

    if (!attrs.file) {
      addError('file', 'Please provide a download link.');
    } else if (is.not.string(attrs.file)) {
      addError('file', 'Please provide a download link as a string.');
    }

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }
}
