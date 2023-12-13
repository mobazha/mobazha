import { guid } from '../../utils';
import { Collection } from 'backbone';
import ShippingOption from '../../models/settings/ShippingOption';

export default class extends Collection {
  model(attrs, options) {
    return new ShippingOption({
      _clientID: attrs._clientID || guid(),
      ...attrs,
    }, options);
  }

  modelId(attrs) {
    return attrs._clientID;
  }
}
