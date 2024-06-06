import { guid } from '../../utils';
import { Collection } from 'backbone';
import OptionalFeature from '../../models/listing/OptionalFeature';

export default class extends Collection {
  model(attrs, options) {
    return new OptionalFeature({
      featureID: attrs.featureID || guid(),
      ...attrs,
    }, options);
  }

  modelId(attrs) {
    return attrs.featureID;
  }
}
