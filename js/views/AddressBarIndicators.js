/* eslint-disable class-methods-use-this */
import * as isIPFS from 'is-ipfs';
import app from '../app';
import loadTemplate from '../utils/loadTemplate';
import baseVw from './baseVw';

export default class extends baseVw {
  constructor(options = {}) {
    super(options);

    this._state = {
      hide: true,
      ...options.initialState || {},
    };
  }

  className() {
    return 'addressBarIndicators';
  }

  updateVisibility(addressBarText) {
    if (typeof addressBarText !== 'string') {
      throw new Error('Please provide a valid address bar as string.');
    }

    const viewOnWebState = {
      hide: true,
    };

    const urlParts = this.getUrlParts(addressBarText);

    if (urlParts.length > 1 && isIPFS.multihash(urlParts[0])) {
      const supportedPages = ['store', 'home', 'followers', 'following'];
      const currentPage = urlParts[1];

      if (supportedPages.includes(currentPage)) {
        const obDotCom = `https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info`;
        const peerID = urlParts[0];

        if (currentPage === 'store') {
          // app: '/peerID/store/' => web: '/store/peerID/'
          viewOnWebState.url = `${obDotCom}/profile/${peerID}`;

          if (urlParts.length === 3) {
            // app: '/peerID/store/slug' => web: '/store/peerID/slug'
            const slug = urlParts[2];
            viewOnWebState.url = `${obDotCom}/listing/${peerID}/${slug}`;
          }
        } else {
          // app: '/peerID/(home|followers|following)' =>
          // web: '/store/(home|followers|following)/peerID'
          viewOnWebState.url = `${obDotCom}/profile/${peerID}`;
        }
      }
    }

    viewOnWebState.hide = !viewOnWebState.url;

    this.setState(viewOnWebState);
  }

  getUrlParts(url) {
    if (typeof url !== 'string') {
      throw new Error('Please provide a valid url as a string.');
    }

    const urlParts = url.startsWith('ob://')
      ? url.slice(5).split(' ')[0]
      : url.split(' ')[0];

    return urlParts.split('/');
  }

  render() {
    this.$el.toggleClass('hidePointer', this.getState().hide);

    loadTemplate('addressBarIndicators.html', (t) => {
      this.$el.html(t({
        ...this._state,
      }));
    });

    return this;
  }
}
