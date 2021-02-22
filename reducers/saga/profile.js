import { call, put, takeEvery, select } from 'redux-saga/effects';
import { isEmpty, get, findIndex } from 'lodash';

import { actions as profileActions } from '../profile';
import { actions as profilesActions } from '../profiles';
import { actions as queueActions } from '../profileQueue';
import { actions as failedQueueActions } from '../profileFailedQueue';
import { actions as streamActions } from '../stream';

import { getProfile, setProfile, setCoins } from '../../api/profile';

import {
  getProfileFetchMode,
  getProfiles,
  getProfileQueue,
  getProfileFailedQueue,
} from './sagaSelector';

import { timeSinceInSeconds } from '../../utils/time';

import {I18n} from '../../langs/I18n';

const PROFILE_RECHECK_MINUTES = 10;

function* fetchProfile(action) {
  const { usecache, async } = yield select(getProfileFetchMode);
  const profileQueue = yield select(getProfileQueue);
  let { peerID } = action.payload || {};
  let getLoaded = get(action.payload, 'getLoaded', false);

  if (isEmpty(peerID)) {
    peerID = '';
  }

  if (!isEmpty(peerID)) {
    const profiles = yield select(getProfiles);
    const profileInCache = profiles.profiles[peerID];
    const queueIdx = findIndex(profileQueue, pfID => pfID === peerID);
    getLoaded = getLoaded && (!isEmpty(profileInCache) || queueIdx >= 0);
  }

  if (!getLoaded) {
    if (!isEmpty(peerID)) {
      // check failed profile queue and if it failed within the last 1 hr, gracefully return
      const profileFailedQueue = yield select(getProfileFailedQueue);
      const timestamp = profileFailedQueue[peerID];
      if (timestamp) {
        if ((timeSinceInSeconds(new Date(timestamp)) < PROFILE_RECHECK_MINUTES * 60)) {
          return;
        } else {
          yield put({ type: failedQueueActions.removeFromFailedQueue, payload: peerID });
        }
      }

      yield put({ type: queueActions.addToQueue, payload: peerID });
    }

    try {
      const result = yield call(getProfile, peerID, usecache, async);
      if (isEmpty(peerID)) {
        yield put({ type: profileActions.setProfile, payload: result });
      }
      if (result) {
        yield put({ type: profilesActions.setProfiles, payload: [result] });
      } else if (!isEmpty(peerID)) {
        yield put({ type: failedQueueActions.addToFailedQueue, payload: peerID });
      }
      yield put({ type: profileActions.setProfileLoading, payload: false });
    } catch (err) {
      // console.log(`Fetch profile error: ${err}`);
    } finally {
      if (!isEmpty(peerID)) {
        yield put({ type: queueActions.removeFromQueue, payload: peerID });
      }
    }
  }
}

function* setUserProfile(action) {
  const { data, onFailure } = action.payload;
  try {
    const result = yield call(setProfile, data, onFailure);
    if (get(result, 'success', true)) {
      yield put({ type: streamActions.updateUser, payload: { profile: result } });
      yield put({ type: profileActions.setProfile, payload: { ...result, loaded: true } });
      const { onSuccess } = action.payload;
      if (onSuccess) {
        onSuccess();
      }
    } else {
      if (onFailure) {
        onFailure(result.reason);
      } else {
        yield put({ type: profileActions.setProfileFailure, error: result.reason });
      }
    }
  } catch (err) {
    const msg = err.message || err;
    if (onFailure) {
      onFailure(msg);
    } else {
      yield put({ type: profileActions.setProfileFailure, error: msg });
    }
  }
}

function* setAcceptedCoins(action) {
  const { coins, onSuccess, onFailure } = action.payload;
  try {
    const result = yield call(setCoins, coins);
    if (get(result, 'success', true)) {
      yield put({
        type: profileActions.setProfile,
        payload: {
          currencies: coins,
          loaded: true,
          updateCoins: true,
        },
      });
      if (onSuccess) {
        onSuccess();
      }
    } else {
      if (onFailure) {
        onFailure(result.reason);
      } else {
        yield put({ type: profileActions.setProfileFailure, error: result.reason });
      }
    }
  } catch (err) {
    if (onFailure) {
      onFailure(err.message);
    } else {
      yield put({ type: profileActions.setProfileFailure, error: err.message });
    }
  }
}

function* fetchProfiles(action) {
  const { usecache, async } = yield select(getProfileFetchMode);
  const profileFailedQueue = yield select(getProfileFailedQueue);

  const profileQueue = yield select(getProfileQueue);
  const profiles = yield select(getProfiles);

  const { peerIDs, getLoaded = true } = action.payload;
  yield put({ type: queueActions.addBatchToQueue, payload: peerIDs });

  let i;
  for (i = 0; i < peerIDs.length; i += 1) {
    const peerID = peerIDs[i];
    const profileInCache = profiles.profiles[peerID];
    const queueIdx = findIndex(profileQueue, pfID => pfID === peerID);
    const inCahceOrQueue = profileInCache || queueIdx >= 0;

    if (!getLoaded || !inCahceOrQueue) {
      // check failed profile queue and if it failed within the last 1 hr, gracefully return
      const timestamp = profileFailedQueue[peerID];
      if (timestamp) {
        if ((timeSinceInSeconds(new Date(timestamp)) < PROFILE_RECHECK_MINUTES * 60)) {
          yield put({ type: queueActions.removeFromQueue, payload: peerID });
          continue;
        } else {
          yield put({ type: failedQueueActions.removeFromFailedQueue, payload: peerID });
        }
      }

      try {
        const result = yield call(getProfile, peerID, usecache, async);
        if (result) {
          yield put({ type: profilesActions.setProfiles, payload: [result] });
        } else {
          yield put({ type: failedQueueActions.addToFailedQueue, payload: peerID });
        }
      } catch (err) {
        // console.log(err);
      } finally {
        yield put({ type: queueActions.removeFromQueue, payload: peerID });
      }
    }
  }
}

const ProfileSaga = function* Search() {
  yield takeEvery(profileActions.fetchProfile, fetchProfile);
  yield takeEvery(profileActions.updateProfile, setUserProfile);
  yield takeEvery(profileActions.updateAcceptedCoins, setAcceptedCoins);
  yield takeEvery(profilesActions.fetchProfiles, fetchProfiles);
};

export default ProfileSaga;
