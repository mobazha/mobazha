import app from '../../app.js';
import BaseModel from '../BaseModel.js';
import Services from '../../collections/Services.js';
import is from 'is_js';

export default class extends BaseModel {
  defaults() {
    return {
      id: -1,
      name: '',
      type: 'FIXED_PRICE',
      currency: app.settings.get('localCurrency'),
      serviceType: 'SAME_WEIGHT_SAME_FEE',
      regions: [],
      services: new Services(),
    };
  }

  get idAttribute() {
    return '_clientID';
  }

  get nested() {
    return {
      services: Services,
    };
  }

  get shippingTypes() {
    return [
      'LOCAL_PICKUP',
      'FIXED_PRICE',
    ];
  }

  validate(attrs) {
    let errObj = {};
    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    if (this.shippingTypes.indexOf(attrs.type) === -1) {
      addError('type', 'The shipping type is not one of the available types.');
    }

    if (is.not.string(attrs.name)) {
      addError('name', 'Please provide a name as a string.');
    } else if (!attrs.name) {
      addError('name', app.polyglot.t('shippingOptionModelErrors.provideName'));
    }

    // todo: check that the regions provided contain valid country codes
    // from our countries module
    if (!attrs.regions || !attrs.regions.length) {
      addError('regions', app.polyglot.t('shippingOptionModelErrors.provideRegion'));
    }

    if (attrs.type !== 'LOCAL_PICKUP' && (!attrs.services || !attrs.services.length)) {
      addError('services', app.polyglot.t('shippingOptionModelErrors.provideService'));
    }

    errObj = this.mergeInNestedErrors(errObj);

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }

  sync(method, model, options) {
    if (method !== 'read' && method !== 'delete') {
      // If all countries are individually provided as shipping regions, we'll send
      // 'ALL' to the server.
      if (_.isEqual(Object.keys(getIndexedCountries()), options.attrs.regions)) {
        options.attrs.regions = ['ALL'];
      }
    }

    return super.sync(method, model, options);
  }

  parse(response = {}) {
    // If the shipping regions are set to 'ALL', we'll replace with a list of individual
    // countries, which is what our UI is designed to work with.
    if (response.regions && response.regions.length && response.regions[0] === 'ALL') {
      response.regions = Object.keys(getIndexedCountries());
    }

    if (response.services && response.services.length) {
      response.services.forEach((service) => {
        service.currency = response.currency;
      });
    }

    return response;
  }
}
