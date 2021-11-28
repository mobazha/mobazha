import _ from 'lodash';
import * as qs from 'query-string';

// const BITCOIN_REGEX = /^([13][a-km-zA-HJ-NP-Z1-9]{25,34})|^((bitcoincash:)?(q|p)[a-z0-9]{41})|^((BITCOINCASH:)?(Q|P)[A-Z0-9]{41})$/;

// reference: https://stackoverflow.com/questions/48874571/what-does-the-new-bch-regex-mean
const BITCOINCASH_REGEX = /^((bitcoincash:)?(q|p)[a-z0-9]{41})|^((BITCOINCASH:)?(Q|P)[A-Z0-9]{41})$/;

// reference: https://stackoverflow.com/questions/21683680/regex-to-match-bitcoin-addresses
const BITCOIN_REGEX = /^(bitcoin:)?[13][a-km-zA-HJ-NP-Z1-9]{25,34}$/;

// reference: https://www.reddit.com/r/zec/comments/8mxj6x/simple_regex_to_validate_a_zcash_tz_address/
const ZCASH_REGEX = /^t1[a-zA-Z0-9]{33}$/;

// reference: https://stackoverflow.com/questions/23570080/how-to-determine-if-litecoin-address-is-valid
const LITECOIN_REGEX = /^[LM3][a-km-zA-HJ-NP-Z1-9]{26,33}$/;

const ETHER_REGEX = /^0x[a-fA-F0-9]{40}$/;

const CONFLUX_REGEX = /^cfx.*$/;

export const UNIVERSAL_LINK_PREFIX = 'https://mobazha.info';

export const getNavRouteAndParamsFromURL = (initialUrl) => {
  if (!initialUrl) {
    return null;
  }

  if (BITCOINCASH_REGEX.test(initialUrl)) {
    return {
      route: 'SendMoney',
      params: {
        coin: 'BCH',
        address: initialUrl,
      },
    };
  } else if (BITCOIN_REGEX.test(initialUrl)) {
    return {
      route: 'SendMoney',
      params: {
        coin: 'BTC',
        address: initialUrl,
      },
    };
  } else if (ZCASH_REGEX.test(initialUrl)) {
    return {
      route: 'SendMoney',
      params: {
        coin: 'ZEC',
        address: initialUrl,
      },
    };
  } else if (LITECOIN_REGEX.test(initialUrl)) {
    return {
      route: 'SendMoney',
      params: {
        coin: 'LTC',
        address: initialUrl,
      },
    };
  } else if (ETHER_REGEX.test(initialUrl)) {
    return {
      route: 'SendMoney',
      params: {
        coin: 'ETH',
        address: initialUrl,
      },
    };
  } else if (CONFLUX_REGEX.test(initialUrl)) {
    return {
      route: 'SendMoney',
      params: {
        coin: 'CFX',
        address: initialUrl,
      },
    };
  } else {
    // Remove http(s) scheme AND domain for universal links.
    const url = initialUrl.replace(/^https?:\/\/[^/]+/, '');

    // Normalize the path, removing leading/trailing/duplicate slashes.
    // For our own scheme, we KEEP the "domain" as part of the path.
    const path = url.replace(/^(?:\w+:)?\/+|\/(?=\/)|\/+$/g, '');

    // If there is no path, then the app should open to wherever it wants.
    if (path) {
      // Always add leading and trailing slashes (removed above).
      // alert(`/${path}/`);

      const items = path.split('/');
      if (items.length > 0) {
        if (items[1] === 'store') {
          if (items.length === 2) {
            return {
              route: 'ExternalStore',
              params: { peerID: items[0] },
            };
          } else if (items.length === 3) {
            return {
              route: 'Listing',
              params: { slug: items[2], peerID: items[0] },
            };
          }
        } else if (items[1] === 'posts') {
          return {
            route: 'FeedDetail',
            params: { activityId: items[2] },
          };
        } else if (items[0] === 'listings' && items[1].startsWith('search?')) {
          return {
            route: 'QueryResult',
            params: { query: items[1].slice(7) },
          };
        } else if (items[0] === 'screens') {
          switch (items[1]) {
            case 'feed':
              return { route: 'Feed' };
            case 'wallet':
              return { route: 'Wallet' };
            default:
              return null;
          }
        }
      }
    }
  }
  return null;
};

export const createStoreUrlFromPeerID = peerID => `${UNIVERSAL_LINK_PREFIX}/profile/${peerID}`;
export const createListingUrlFromPeerIDAndSlug = (peerID, slug) => `${UNIVERSAL_LINK_PREFIX}/listing/${peerID}/${slug}/`;
export const createFeedUrlFromPeerIDAndSlug = (peerID, slug) => `${UNIVERSAL_LINK_PREFIX}/posts/${peerID}/${slug}`;

export const getRouteNameFromState = (state) => {
  if (_.isEmpty(state.routes)) {
    return state.routeName;
  }
  const idx = state.index;
  return getRouteNameFromState(state.routes[idx]);
};

export const getCurrentRouteParamFromState = (state) => {
  if (_.isEmpty(state.routes)) {
    return state.params || {};
  }
  const idx = state.index;
  return getCurrentRouteParamFromState(state.routes[idx]);
};

const checkCoinAndAmount = (deeplink) => {
  const regexp = new RegExp(/(bitcoin|bitcoincash|litecoin|zcash|ethereum):(.*)\?amount=(.*)/g);
  const elements = regexp.exec(deeplink);
  if (!elements || elements.length < 4) {
    return { link: deeplink };
  }

  return {
    // link: ['bitcoin', 'bitcoincash'].includes(elements[1]) ? `${elements[1]}:${elements[2]}` : elements[2],
    link: elements[2],
    amount: +elements[3],
  };
};

export const handleOBDeeplinkWithNavigation = (deeplink, navigation) => {
  const { link, amount } = checkCoinAndAmount(deeplink);
  const { route, params } = getNavRouteAndParamsFromURL(link) || {};
  // console.warn('--link, amount, route, params--', link, amount, route, params);
  if (!route) {
    return;
  }

  if (amount) {
    navigation.push(route, { ...params, amount });
  } else {
    navigation.push(route, params);
  }
};
