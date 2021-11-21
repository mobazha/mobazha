import { takeEvery, select, call, put } from 'redux-saga/effects';
import * as _ from 'lodash';

import { actions } from '../moderators';
import { actions as settingActions } from '../settings';
import { actions as appStateActions } from '../appstate';

import { actions as profilesActions } from '../profiles';
import { getStoreModerators, getIsFirstETHInit, getWalletList } from './sagaSelector';
import { getModerators } from '../../api/moderators';
import { actions as profileActions } from '../profile';
import { patchSettingsRequest as patchSettingsRequestSaga } from './settings';

function* fetchModerators() {
  const storeModerators = yield select(getStoreModerators);

  try {
    const { moderators } = yield call(getModerators);
    if (moderators) {
      const moderatorIDs = moderators.map(moderator => moderator.peerID);
      yield put({ type: actions.setModerators, payload: moderatorIDs });
      yield put({
        type: profilesActions.fetchProfiles,
        payload: {
          peerIDs: moderatorIDs,
          getLoaded: false,
        },
      });

      // ETH initialization / migration
      const isFirstETHInit = yield select(getIsFirstETHInit);
      const currencies = yield select(getWalletList);
      if (isFirstETHInit && !currencies.includes('ETH')) { // Needs listings migrated into ETH
        yield call(patchSettingsRequestSaga, { payload: { storeModerators: moderatorIDs } });
        // yield put({ type: profileActions.updateAcceptedCoins, payload: { coins: [...currencies, 'ETH'] } });
      } else if (!_.isEqual(storeModerators, moderatorIDs)) {
        yield put({ type: settingActions.patchSettingsRequest, payload: { storeModerators: moderatorIDs } });
      }
      yield put({ type: appStateActions.setETHBuildMigrated, payload: true });
    }
  } catch (err) {
    console.warn(`Error fetching moderators. Error: ${err}`);
  }
}

const ModeratorsSaga = function* ModeratorsSaga() {
  yield takeEvery(actions.fetchModerators, fetchModerators);
};

export default ModeratorsSaga;
