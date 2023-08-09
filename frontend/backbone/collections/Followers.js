/* Used for lists of followers and following */

import app from '../app';
import { Collection } from 'backbone';
import Follower from '../models/Follower';

export default class extends Collection {
  constructor(models = [], options = {}) {
    super(models, options);

    const types = ['followers', 'following'];
    if (types.indexOf(options.type) === -1) {
      throw new Error(`Please provide a type as one of ${types.join(', ')}`);
    }

    if (!options.peerID) {
      throw new Error('Please provide a peerID');
    }

    this.options = options;
  }

  model(attrs, options) {
    return new Follower(attrs, options);
  }

  modelId(attrs) {
    return attrs.peerID;
  }

  url() {
    return app.getServerUrl(`ob/${this.options.type === 'followers' ? 'followers' : 'following'}` +
      `${app.profile.id === this.options.peerID ? '' : `/${this.options.peerID}`}`);
  }

  parse(response) {
    return response.map(peerID => ({ peerID }));
  }
}
