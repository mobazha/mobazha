import { Collection } from 'backbone';
import { guid } from '../../utils';
import Image from '../../models/listing/Image';

export default class ListingImages extends Collection {
  model(attrs, options) {
    return new Image({
      _clientID: attrs._clientID || guid(),
      ...attrs,
    }, options);
  }

  modelId(attrs) {
    return attrs._clientID;
  }
}