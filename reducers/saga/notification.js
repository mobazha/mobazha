import { delay } from 'redux-saga/effects';
import { call, put, takeEvery, select } from 'redux-saga/effects';
import { isEmpty, get } from 'lodash';

import { actions } from '../notifications';
import { actions as socketActions } from '../socket';
import { getUserCredentails } from './sagaSelector';
import { fetchNotifications, markAsRead, markAsReadAll } from '../../api/notifications';

function* getNotificationCount() {
  const { username, password } = yield select(getUserCredentails);
  try {
    const result = yield call(fetchNotifications, username, password, '', '');
    yield put({ type: actions.setCount, payload: result });
  } catch (err) {
    console.log('Get Notification Error', err);
  }
  yield delay(60 * 1000);
  yield put({ type: actions.getCount });
}

function* fetchData(action) {
  const { username, password } = yield select(getUserCredentails);
  const limit = get(action.payload, 'limit', '');
  const offsetId = get(action.payload, 'offsetId', '');
  const result = yield call(fetchNotifications, username, password, limit, offsetId);
  if (isEmpty(offsetId)) {
    yield put({ type: actions.setNotifications, payload: result });
  } else {
    yield put({ type: actions.appendNotifications, payload: result });
  }
}

function* fetchLatestData(action) {
  const { username, password } = yield select(getUserCredentails);
  const limit = get(action.payload, 'limit', '');
  const offsetId = get(action.payload, 'offsetId', '');
  const result = yield call(fetchNotifications, username, password, limit, offsetId);
  yield put({ type: actions.prependNotifications, payload: result });
}

function* markOne(action) {
  const { username, password } = yield select(getUserCredentails);
  yield call(markAsRead, username, password, action.payload);
}

function* markAll() {
  const { username, password } = yield select(getUserCredentails);
  yield call(markAsReadAll, username, password);
}

function* incomingSocketMessage(action) {
  const { payload } = action;
  const { notification } = payload || {};
  if (notification && notification.type !== 'payment') {
    yield put(actions.getCount());
  }
}

const NotificationSaga = function* Notification() {
  yield takeEvery(actions.getCount, getNotificationCount);
  yield takeEvery(actions.fetchNotifications, fetchData);
  yield takeEvery(actions.fetchLatestNotifications, fetchLatestData);
  yield takeEvery(actions.markNotificationAsRead, markOne);
  yield takeEvery(actions.markAllAsRead, markAll);
  yield takeEvery(socketActions.incomingSocketMessage, incomingSocketMessage);
};

export default NotificationSaga;
