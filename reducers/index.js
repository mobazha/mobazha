import { persistCombineReducers } from 'redux-persist';
import storage from 'redux-persist/lib/storage';
import { createFilter, createBlacklistFilter } from 'redux-persist-transform-filter';

import nav from './navigation';
import product from './product';
import search from './search';
import appstate from './appstate';
import profile from './profile';
import moderators from './moderators';
import follow from './follow';
import followQueue from './followQueue';
import createListing from './createListing';
import notifications from './notifications';
import settings from './settings';
import moderationSettings from './moderationSettings';
import chat from './chat';
import feed from './feed';
import stream from './stream';
import profileQueue from './profileQueue';
import profileFailedQueue from './profileFailedQueue';
import profiles from './profiles';
import orders from './order';
import exchangeRate from './exchangeRate';
import wallet from './wallet';
import listings from './listings';
import wishlist from './wishlist';
import featured from './featured';
import promo from './promo';
import storeListings from './storeListings';
import config from './config';
import ratings from './ratings';

import {I18n} from '../langs/I18n';

const loadPartialOrderFilter = createFilter(
  'orders',
  null,
  ['processing'],
  'blacklist',
);

const loadPartialAppstateFilter = createFilter(
  'appstate',
  null,
  ['onboardingStatus'],
  'blacklist',
);

const removeSettingsErrorsFilter = createBlacklistFilter('settings', ['settingsError']);

const removeProfileErrorsFilter = createBlacklistFilter('profile', ['profileError']);

const disableParialFeedFilter = createBlacklistFilter('feed', ['globalList', 'localList', 'feeds']);

const removepPollForSessionExecutedFilter = createBlacklistFilter('orders', ['pollForSessionExecuted']);

const persistConfig = {
  key: 'root',
  storage,
  blacklist: [
    'search',
    'stream',
    'exchangeRate',
    'createListing',
    'nav',
    'profileQueue',
    'followQueue',
    'ratings',
    'featured',
  ],
  transforms: [
    loadPartialOrderFilter,
    loadPartialAppstateFilter,
    disableParialFeedFilter,
    removeSettingsErrorsFilter,
    removeProfileErrorsFilter,
    removepPollForSessionExecutedFilter,
  ],
};

export default persistCombineReducers(persistConfig, {
  nav,
  product,
  search,
  appstate,
  profile,
  follow,
  followQueue,
  createListing,
  notifications,
  settings,
  moderationSettings,
  chat,
  profileQueue,
  profiles,
  profileFailedQueue,
  orders,
  feed,
  stream,
  exchangeRate,
  wallet,
  listings,
  wishlist,
  featured,
  promo,
  storeListings,
  moderators,
  config,
  ratings,
});
