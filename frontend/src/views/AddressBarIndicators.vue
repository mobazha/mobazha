<template>
  <div :class="`addressBarIndicators ${ob.hide ? 'hidePointer' : ''}`">
    <div class="viewOnWebContainer clrP" v-show="!ob.hide">
      <a class="txU clrTEmph1 lgHtBx" :href="ob.url">
        <div class="viewOnWebText">{{ ob.polyT('editListing.viewListingOnWebLink') }}</div>
        <i class="ion-android-open clrTEmph1"></i>
      </a>
    </div>
    <div class="torIndicatorContainer clrP">
      <div class="torIndicator toolTipNoWrap txCtr" :data-tip="ob.polyT('pageNav.torOnTooltip')"></div>
    </div>

  </div>
</template>

<script>
import * as isIPFS from 'is-ipfs';
import app from '../../backbone/app';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this._state = {
        hide: true,
        ...options.initialState || {},
      };
    },

    updateVisibility (addressBarText) {
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
    },

    getUrlParts (url) {
      if (typeof url !== 'string') {
        throw new Error('Please provide a valid url as a string.');
      }

      const urlParts = url.startsWith('ob://')
        ? url.slice(5).split(' ')[0]
        : url.split(' ')[0];

      return urlParts.split('/');
    },

  }
}
</script>
<style lang="scss" scoped></style>
