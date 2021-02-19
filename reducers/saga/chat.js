import { call, put, delay, takeEvery, takeLatest, select } from 'redux-saga/effects';
import * as _ from 'lodash';

import { actions as chatActions } from '../chat';
import { actions as StreamActions } from '../stream';
import { actions as batchProfileActions } from '../profiles';

import * as chatApi from '../../api/chat';

import { getUserCredentails } from './sagaSelector';
import { eventTracker } from '../../utils/EventTracker';

export function* getChats() {
  const { username, password } = yield select(getUserCredentails);
  try {
    const chats = yield call(chatApi.fetchChats, username, password);
    if (chats.length >= 0) {
      yield put({ type: chatActions.setChats, payload: chats });
      yield put({
        type: batchProfileActions.fetchProfiles,
        payload: { peerIDs: chats.map(chat => chat.peerId) },
      });
    }
  } catch (err) {
    console.log('Chats Saga Error: ', err);
  }
  yield put({ type: chatActions.setChatsLoading, payload: false });
}

export function* updateChats() {
  const { username, password } = yield select(getUserCredentails);
  try {
    const chats = yield call(chatApi.fetchChats, username, password);
    if (chats.length >= 0) {
      yield put({ type: chatActions.setChats, payload: chats });
      yield put({
        type: batchProfileActions.fetchProfiles,
        payload: { peerIDs: chats.map(chat => chat.peerId) },
      });
    }
  } catch (err) {
    console.log('Chats Saga Error: ', err);
  }
  yield delay(60 * 1000);
  yield put({ type: chatActions.updateChats });
}

export function* getChatDetail(action) {
  const { username, password } = yield select(getUserCredentails);
  const { peerID, subject, isFromOrder } = action.payload;
  try {
    const result = yield call(
      chatApi.fetchChatDetail,
      username,
      password,
      isFromOrder ? null : peerID,
      subject,
    );
    yield put({
      type: chatActions.setChatDetailLoading,
      payload: false,
    });
    yield put({
      type: chatActions.setChatDetail,
      payload: {
        chatDetails: result,
        peerID,
        subject,
      },
    });
  } catch (err) {
    console.log('ChatDetail Saga Error: ', err);
  }
}

export function* sendChat(action) {
  const { username, password } = yield select(getUserCredentails);
  const {
    peerID, message, localId, onSent, subject = '', orderType,
  } = action.payload;
  try {
    // Send message request to backend
    if (message !== '') {
      eventTracker.trackEvent('Chat-MessageSent');
    }
    const result = yield call(
      chatApi.sendChat,
      username,
      password,
      peerID,
      subject,
      message,
      orderType,
    );
    if (message !== '') {
      // If the message is successfully sent, then set the message info saved in local to success
      // and updates the state in the component to success
      eventTracker.trackEvent('Chat-MessageDelivered');
      yield put({ type: chatActions.updateLocalInfo, payload: { localId, result, success: _.get(result, 'success', true) } });
      yield put({ type: chatActions.fetchChats });
      if (onSent) {
        yield call(onSent, _.get(result, 'success', true));
      }
    }
  } catch (err) {
    if (message !== '') {
      // If the message is failed to sent, then set the message info saved in local to failure
      // and updates the state in the component to failure
      eventTracker.trackEvent('Chat-MessageFailed', { reason: err.reason });
      yield put({ type: chatActions.updateLocalInfo, payload: { localId, success: false } });
      if (onSent) {
        yield call(onSent, false);
      }
    }
  }
}

export function* sendNotification(peerID, orderId, orderType) {
  yield put({ type: StreamActions.sendNotification,
    payload: {
      verb: 'chat',
      type: 'sent',
      peerID,
      content: { body: 'You have received a message!', orderId, orderType },
    } });
}

export function* sendChatNotification(action) {
  const { peerID, message, subject: orderId, orderType: myOrderType } = action.payload;
  if (_.isEmpty(message)) {
    return;
  }

  let orderType = myOrderType === 'purchases' ? 'sales' : 'purchases';

  if (typeof peerID === 'string') {
    yield call(sendNotification, peerID, orderId, orderType);
  } else {
    for (let i = 0; i < peerID.length; i += 1) {
      yield call(sendNotification, peerID[i], orderId, orderType);
    }
  }
}

export function* setChatAsRead(action) {
  const { username, password } = yield select(getUserCredentails);
  const { peerID, subject, isFromOrder } = action.payload;
  try {
    yield call(
      chatApi.setChatAsRead,
      username,
      password,
      isFromOrder ? null : peerID,
      subject,
    );
  } catch (err) {
    console.log('Saga Error: Set Chat As Read Errors');
  }
}

export function* deleteChatConversation(action) {
  const { username, password } = yield select(getUserCredentails);
  const peerID = action.payload;
  try {
    yield call(
      chatApi.deleteChatConversation,
      username, password, peerID,
    );
    yield put({ type: chatActions.fetchChats });
  } catch (err) {
    console.log('Saga Error: Delete Chat Conversation');
  }
}

export default function* Chat() {
  yield takeEvery(chatActions.fetchChats, getChats);
  yield takeEvery(chatActions.updateChats, updateChats);
  yield takeEvery(chatActions.fetchChatDetail, getChatDetail);
  yield takeEvery(chatActions.sendChat, sendChat);
  yield takeLatest(chatActions.sendChat, sendChatNotification);
  yield takeEvery(chatActions.setChatAsRead, setChatAsRead);
  yield takeEvery(chatActions.deleteChatConversation, deleteChatConversation);
}
