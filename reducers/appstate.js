import { createAction, handleActions } from 'redux-actions';
import { findIndex, concat, takeRight, filter } from 'lodash';

import { changeTrackingStatus } from '../utils/EventTracker';

export const actions = {
  setKeyword: 'APPSTATE/SET_KEYWORD',
  removeKeyword: 'APPSTATE/REMOVE_KEYWORD',
  setCategory: 'APPSTATE/SET_CATEGORY',
  setCategoryKeyword: 'APPSTATE/SET_CATEGORY_KEYWORD',
  setShippingAddress: 'APPSTATE/SET_SHIPPINGADDRESS',
  setShippingOption: 'APPSTATE/SET_SHIPPINGOPTION',
  clearShippingOption: 'APPSTATE/CLEAR_SHIPPINGOPTION',
  setPaymentMethod: 'APPSTATE/SET_PAYMENTMETHOD',
  setCheckoutNote: 'APPSTATE/SET_CHECKOUTNOTE',
  clearCheckoutNote: 'APPSTATE/CLEAR_CHECKOUTNOTE',
  setReceiveMoneyCoin: 'APPSTATE/SET_RECEIVEMONEYCOIN',
  setSendMoneyCoin: 'APPSTATE/SET_SENDMONEYCOIN',
  setModerator: 'APPSTATE/SET_MODERATOR',
  setTags: 'APPSTATE/SET_TAGS',
  publishLocalData: 'APPSTATE/PUBLISH_LOCAL_DATA',
  localDataPublished: 'APPSTATE/LOCAL_DATA_PUBLISHED',
  showPanel: 'APPSTATE/SHOW_PANEL',
  hidePanel: 'APPSTATE/HIDE_PANEL',
  setOnboardingStatus: 'APPSTATE/SET_ONBOARDING_STATUS',
  initializeLogin: 'APPSTATE/INITIALIZE_LOGIN',
  setTrackingStatus: 'APPSTATE/SET_TRACKING_STATUS',
  appInstalled: 'APPSTATE/APP_INSTALLED',
  updateLastReviewPrompt: 'APPSTATE/UPDATE_LAST_REVIEW_PROMPT',
  triggerReviewPrompt: 'APPSTATE/TRIGGER_REVIEW_PROMPT',
  setGuest: 'APPSTATE/SET_GUEST',
  copiedBackupPhrase: 'APPSTATE/COPIED_BACKUP_PHRASE',
  updateNotificationSettings: 'APPSTATE/UPDATE_NOTIFICATION_SETTINGS',
  setStreamBuildNotificationSeen: 'APPSTATE/SET_STREAM_BUILD_NOTIFICATION_SEEN',
  setEULAPopupSeen: 'APPSTATE/SET_EULA_POPUP_SEEN',
  setStreamBuildMigrated: 'APPSTATE/SET_STREAM_BUILD_MIGRATED',
  setETHBuildMigrated: 'APPSTATE/SET_ETH_BUILD_MIGRATED',
  migrateFeatureNotificationSettings: 'APPSTATE/MIGRATE_FEATURE_NOTIFICATION_SETTINGS',
  showToast: 'APPSTATE/SHOW_TOAST',
  hideToast: 'APPSTATE/HIDE_TOAST',
};

export const setKeyword = createAction(actions.setKeyword);
export const removeKeyword = createAction(actions.removeKeyword);
export const setTags = createAction(actions.setTags);
export const setCategory = createAction(actions.setCategory);
export const setCategoryKeyword = createAction(actions.setCategoryKeyword);
export const setShippingAddress = createAction(actions.setShippingAddress);
export const setShippingOption = createAction(actions.setShippingOption);
export const clearShippingOption = createAction(actions.clearShippingOption);

export const setPaymentMethod = createAction(actions.setPaymentMethod);
export const setCheckoutNote = createAction(actions.setCheckoutNote);
export const clearCheckoutNote = createAction(actions.clearCheckoutNote);
export const setReceiveMoneyCoin = createAction(actions.setReceiveMoneyCoin);
export const setSendMoneyCoin = createAction(actions.setSendMoneyCoin);
export const setModerator = createAction(actions.setModerator);
export const publishLocalData = createAction(actions.publishLocalData);

export const showPanel = createAction(actions.showPanel);
export const hidePanel = createAction(actions.hidePanel);
export const setOnboardingStatus = createAction(actions.setOnboardingStatus);
export const initializeLogin = createAction(actions.initializeLogin);

export const setGuest = createAction(actions.setGuest);

export const setTrackingStatus = createAction(actions.setTrackingStatus);

export const appInstalled = createAction(actions.appInstalled);
export const triggerReviewPrompt = createAction(actions.triggerReviewPrompt);

export const copiedBackupPhrase = createAction(actions.copiedBackupPhrase);
export const updateNotificationSettings = createAction(actions.updateNotificationSettings);

export const setStreamBuildNotificationSeen = createAction(actions.setStreamBuildNotificationSeen);
export const setEULAPopupSeen = createAction(actions.setEULAPopupSeen);
export const setStreamBuildMigrated = createAction(actions.setStreamBuildMigrated);
export const setETHBuildMigrated = createAction(actions.setETHBuildMigrated);

export const hideToast = createAction(actions.hideToast);
export const showToast = createAction(actions.showToast);

const initialState = {
  main_status: 'foreground',
  keyword: '',
  category: '',
  recent_searches: [],
  recent_tags: [],
  username: 'username',
  password: 'password',
  async: false,
  usecache: true,
  shippingAddress: 0,
  shippingOption: {},
  paymentMethod: 'BTC',
  checkoutNote: '',
  receiveMoneyCoin: 'BTC',
  sendMoneyCoin: 'BTC',
  feeLevel: {},
  moderator: '',
  termsAndConditions: '',
  returnPolicy: '',
  currentPanel: null,
  onboardingStatus: 'loading',
  isTrackingEvent: false,
  appInstalled: null,
  lastReviewPrompt: null,
  guest: undefined,
  isBackupCopied: false,
  notifications: undefined,
  streamBuildNotificationSeen: false,
  EULAPopupSeen: true,
  streamBuildMigrated: false,
  ETHBuildMigrated: false,
  toast: {
    visible: false,
    type: '',
    message: '',
  },
};

export default handleActions(
  {
    [actions.setKeyword]: (state, action) => {
      const keyword = action.payload;
      if (keyword === '') {
        return { ...state, keyword: '', category: '' };
      }
      const recentSearches = state.recent_searches;
      const idx = findIndex(recentSearches, o => o === keyword);
      if (idx === -1) {
        return {
          ...state,
          keyword,
          recent_searches: concat([keyword], recentSearches),
          category: '',
        };
      }
      return { ...state, keyword, category: '' };
    },
    [actions.setCategoryKeyword]: (state, { payload }) => ({
      ...state, category: payload.category, keyword: payload.keyword,
    }),
    [actions.setCategory]: (state, { payload }) => ({
      ...state, category: payload, keyword: '',
    }),
    [actions.setTrackingStatus]: (state, action) => {
      changeTrackingStatus(action.payload);
      return { ...state, isTrackingEvent: action.payload };
    },
    [actions.removeKeyword]: (state, action) => {
      const keyword = action.payload;
      const recent_searches = state.recent_searches.filter(word => word !== keyword);
      return { ...state, recent_searches };
    },
    [actions.setTags]: (state, action) => {
      const newTags = action.payload;
      const recentTags = state.recent_tags;
      const filteredTags = filter(
        newTags,
        tag => findIndex(recentTags, recentTag => recentTag === tag) === -1,
      );
      const newList = [...recentTags, ...filteredTags];
      return {
        ...state,
        recent_tags: takeRight(newList, 10),
      };
    },
    [actions.setShippingAddress]: (state, action) => ({
      ...state,
      shippingAddress: action.payload,
    }),
    [actions.setShippingOption]: (state, action) => ({
      ...state,
      shippingOption: action.payload,
    }),
    [actions.clearShippingOption]: state => ({
      ...state,
      shippingOption: {},
    }),
    [actions.setPaymentMethod]: (state, action) => ({
      ...state,
      ...action.payload,
    }),
    [actions.setCheckoutNote]: (state, action) => ({
      ...state,
      checkoutNote: action.payload,
    }),
    [actions.clearCheckoutNote]: state => ({
      ...state,
      checkoutNote: '',
    }),
    [actions.setReceiveMoneyCoin]: (state, action) => ({
      ...state,
      receiveMoneyCoin: action.payload,
    }),
    [actions.setSendMoneyCoin]: (state, action) => ({
      ...state,
      sendMoneyCoin: action.payload,
    }),
    [actions.setModerator]: (state, action) => ({
      ...state,
      moderator: action.payload,
    }),
    [actions.showPanel]: (state, action) => ({
      ...state,
      currentPanel: action.payload,
    }),
    [actions.hidePanel]: state => ({
      ...state,
      currentPanel: { ...state.currentPanel, type: null },
    }),
    'SEARCH/CLEAR_FILTER': state => ({
      ...state,
      keyword: '',
    }),
    [actions.setOnboardingStatus]: (state, action) => ({
      ...state,
      onboardingStatus: action.payload,
    }),
    [actions.appInstalled]: (state) => {
      if (!state.appInstalled) {
        console.log({ ...state, appInstalled: new Date() });
        return { ...state, appInstalled: new Date() };
      }
      return state;
    },
    [actions.updateLastReviewPrompt]: state => ({ ...state, lastReviewPrompt: new Date() }),
    [actions.setGuest]: (state, action) => ({ ...state, guest: action.payload }),
    [actions.copiedBackupPhrase]: state => ({ ...state, isBackupCopied: true }),
    [actions.updateNotificationSettings]: (state, action) => {
      const { topic, enable } = action.payload;
      if (topic === 'all') {
        return {
          ...state,
          notifications: {
            promotions: enable,
            announcements: enable,
            giveaways: enable,
            chat: enable,
            orders: enable,
            likes: enable,
            comments: enable,
          },
        };
      }

      return {
        ...state,
        notifications: { ...state.notifications, [topic]: enable },
      };
    },
    [actions.setStreamBuildNotificationSeen]: (state, action) => ({
      ...state,
      streamBuildNotificationSeen: action.payload,
    }),
    [actions.setEULAPopupSeen]: (state, action) => ({
      ...state,
      EULAPopupSeen: action.payload,
    }),
    [actions.setStreamBuildMigrated]: (state, action) => ({
      ...state,
      streamBuildMigrated: action.payload,
    }),
    [actions.setETHBuildMigrated]: (state, action) => ({
      ...state,
      ETHBuildMigrated: action.payload,
    }),
    [actions.migrateFeatureNotificationSettings]: state => ({
      ...state,
      notifications: {
        ...state.notifications,
        chat: true,
        orders: true,
        likes: true,
        comments: true,
      },
    }),
    [actions.showToast]: (state, { payload: { type, message } }) => ({
      ...state,
      toast: {
        visible: true, type, message,
      },
    }),
    [actions.hideToast]: state => ({ ...state,
      toast: {
        ...state.toast,
        visible: false,
      },
    }),
  },
  initialState,
);
