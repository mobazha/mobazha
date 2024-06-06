import { guid } from '../../utils';
import { Collection } from 'backbone';
import Download from '../../models/listing/Download';

export default class extends Collection {
  model(attrs, options) {
    return new Download({
      downloadID: attrs.downloadID || guid(),
      ...attrs,
    }, options);
  }

  modelId(attrs) {
    return attrs.downloadID;
  }
}
