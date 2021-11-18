import { put, takeEvery, select, call } from 'redux-saga/effects';
import moment from 'moment';
import Auth from '@react-native-firebase/auth';
import Messaging from '@react-native-firebase/messaging';
import Reactotron from 'reactotron-react-native';
import AsyncStorage from '@react-native-community/async-storage';
import * as _ from 'lodash';

import { actions } from '../appstate';
import { actions as searchActions } from '../search';
import { actions as walletActions } from '../wallet';

import {
  getUserCredentails,
  getUserPeerID,
  getLastReviewPrompt,
  getAppInstalled,
  getMyProfile,
  getNotificationSettings,
  getIsFirstStreamInit,
} from './sagaSelector';
import { publish, ingestPeer, resolveIpns } from '../../api/publish';
import { getFollowings } from '../../api/follow';
import { rateApp } from '../../utils/rateApp';
import StreamClient from '../../StreamClient';
import { getProfile } from '../../api/profile';
import { addPeerToTokenMap } from '../../utils/firestore';

import { actions as settingActions } from '../settings';
import { actions as profileActions } from '../profile';
import { actions as notificationActions } from '../notifications';
import { actions as exchangeRateActions } from '../exchangeRate';
import { actions as followActions } from '../follow';
import { actions as moderatorActions } from '../moderators';
import { actions as chatActions } from '../chat';
import { actions as streamActions } from '../stream';

function* sendSearchRequest() {
  yield put({ type: searchActions.doSearch });
  yield put({ type: searchActions.doSearchUser });
}

function* sendCategoryRequest() {
  yield put({ type: searchActions.doSearch });
}

function* publishData() {
  const { username, password } = yield select(getUserCredentails);
  yield call(publish, username, password);
}

function* notifyServer() {
  const peerID = yield select(getUserPeerID);
  try {
    const body = yield call(resolveIpns);
    yield call(ingestPeer, peerID, body);
  } catch (err) {
    console.log('ingesting peer failed: ', err);
  }
}

function* triggerRate() {
  const lastPrompt = yield select(getLastReviewPrompt);
  const appInstalled = yield select(getAppInstalled);
  if (lastPrompt) {
    if (moment().diff(moment(lastPrompt), 'months') >= 4) {
      yield call(rateApp);
      yield put({ type: actions.updateLastReviewPrompt });
    }
  } else {
    if (moment().diff(moment(appInstalled), 'days') >= 1) {
      yield call(rateApp);
      yield put({ type: actions.updateLastReviewPrompt });
    }
  }
}

function* setGuest(action) {
  const { country, localCurrency, name } = action.payload;

  yield put({ type: settingActions.patchSettingsRequest, payload: { country, localCurrency } });

  const profile = yield select(getMyProfile);

  yield put({
    type: profileActions.setProfile,
    payload: {
      ...profile,
      name,
    },
  });
}

function* initializeLogin() {
  // try {
  //   const userData = yield call([Auth(), 'signInAnonymously']);
  //   const { user, additionalUserInfo } = userData;
  //   if (true || additionalUserInfo.isNewUser) {
  //     const peerID = yield select(getUserPeerID);
  //     const fcmToken = yield call(AsyncStorage.getItem, 'fcmToken');
  //     // console.warn('-------addPeerToTokenMap-------', user.uid, peerID, fcmToken);
  //     addPeerToTokenMap(user.uid, peerID, fcmToken);
  //   }
  // } catch (e) {
  //   console.warn('-------failed to update-------', e);
  // }

  yield put({ type: profileActions.fetchProfile });
  yield put({ type: notificationActions.getCount });
  yield put({ type: exchangeRateActions.fetchExchangeRate });
  yield put({ type: followActions.fetchFollowers });
  yield put({ type: followActions.fetchFollowings });
  yield put({ type: followActions.fetchFollowingsFromLocal });
  yield put({ type: moderatorActions.fetchModerators });
  yield put({ type: chatActions.updateChats });
  yield put({ type: walletActions.getFees });
  yield put({ type: walletActions.fetchCoinAddresses });

  // // Notification initialization / migration
  // const notifications = yield select(getNotificationSettings);
  // if (!notifications) {
  //   yield put({
  //     type: actions.updateNotificationSettings,
  //     payload: { topic: 'all', enable: true },
  //   });
  // } else if (notifications.chat === undefined) {
  //   yield put({ type: actions.migrateFeatureNotificationSettings });
  // }

  try {
    const profile = yield getProfile();
    // Stream initialization / migration
    const isFirstStreamInit = yield select(getIsFirstStreamInit);
    if (isFirstStreamInit) {
      const followings = yield getFollowings();
      const profiles = _.get(followings, 'profiles', []);
      Reactotron.log('initializing stream started - first time', new Date());
      yield StreamClient.initializeStream(
        profile,
        Array.isArray(profiles) ? profiles.map(profile => profile.peerID) : [],
        isFirstStreamInit,
      );
      yield put({ type: actions.setStreamBuildMigrated, payload: true });
    } else {
      Reactotron.log('initializing stream started - not first time', new Date());
      yield StreamClient.initializeStream(profile);
    }
    yield put({ type: streamActions.streamInitFinished });
    Reactotron.log('initializing stream finished', new Date());
  } catch (err) {
    console.log('stream initialize failed', err);
    // Stream initialization failed
  }
}

function subscribeToFCMTopic(topic) {
  if (false && __DEV__) {
    Messaging().subscribeToTopic(`staging-${topic}`);
  } else {
    Messaging().subscribeToTopic(topic);
  }
}

function unsubscribeFromFCMTopic(topic) {
  if (false && __DEV__) {
    Messaging().unsubscribeFromTopic(`staging-${topic}`);
  } else {
    Messaging().unsubscribeFromTopic(topic);
  }
}

function* updateNotificationSettings(action) {
  const userPeerID = yield select(getUserPeerID);

  const { topic, enable } = action.payload;
  if (topic === 'all') {
    if (enable) {
      subscribeToFCMTopic('promotions');
      subscribeToFCMTopic('giveaways');
      subscribeToFCMTopic('announcements');
      subscribeToFCMTopic(`chat-${userPeerID}`);
      subscribeToFCMTopic(`orders-${userPeerID}`);
      subscribeToFCMTopic(`likes-${userPeerID}`);
      subscribeToFCMTopic(`comments-${userPeerID}`);
    } else {
      unsubscribeFromFCMTopic('promotions');
      unsubscribeFromFCMTopic('giveaways');
      unsubscribeFromFCMTopic('announcements');
      unsubscribeFromFCMTopic(`chat-${userPeerID}`);
      unsubscribeFromFCMTopic(`orders-${userPeerID}`);
      unsubscribeFromFCMTopic(`likes-${userPeerID}`);
      unsubscribeFromFCMTopic(`comments-${userPeerID}`);
    }
  } else {
    if (enable) {
      subscribeToFCMTopic(topic);
    } else {
      unsubscribeFromFCMTopic(topic);
    }
  }
}

function* migrateFeatureNotificationSettings() {
  const userPeerID = yield select(getUserPeerID);
  subscribeToFCMTopic(`chat-${userPeerID}`);
  subscribeToFCMTopic(`orders-${userPeerID}`);
  subscribeToFCMTopic(`likes-${userPeerID}`);
  subscribeToFCMTopic(`comments-${userPeerID}`);
}

const AppstateSaga = function* Search() {
  yield takeEvery(actions.publishLocalData, publishData);
  // yield takeEvery(actions.localDataPublished, notifyServer);
  yield takeEvery(actions.setKeyword, sendSearchRequest);
  yield takeEvery(actions.setCategory, sendCategoryRequest);
  yield takeEvery(actions.setCategoryKeyword, sendCategoryRequest);
  yield takeEvery(actions.triggerReviewPrompt, triggerRate);
  yield takeEvery(actions.setGuest, setGuest);
  yield takeEvery(actions.initializeLogin, initializeLogin);
  // yield takeEvery(actions.updateNotificationSettings, updateNotificationSettings);
  yield takeEvery(actions.migrateFeatureNotificationSettings, migrateFeatureNotificationSettings);
};

export default AppstateSaga;
