import { call, put, delay, takeEvery, select, take } from 'redux-saga/effects';
import { eventChannel } from 'redux-saga';
import moment from 'moment';
import { isEmpty, get } from 'lodash';

import { actions as streamActions } from '../stream';
import appstate, { actions as appstateActions } from '../appstate';

import { getMyProfile, getUserPeerID } from './sagaSelector';
import {
  getFeedCount,
  getFollowings,
  getFirstNotificatyionId,
  getLastNotificatyionId,
  getNotificationCount,
  getUserSort,
  getAllNotifications,
  getActivity,
} from '../../selectors/stream';
import { parseFeedData } from '../../utils/stream';
import StreamClient from '../../StreamClient';
import { convertFromReference } from '../../utils/feed';
import { STREAM_FEED_PAGE_SIZE } from '../../config/stream';

function* fetchStream(action) {
  try {
    const { filter, sort, hashtag, appending, peerID } = action.payload;
    const offset = yield select(getFeedCount(filter, sort));
    // const query = { offset: appending ? offset : 0, ranking: sort ? sort : undefined };
    const query = { offset: appending ? offset : 0 };
    if (!StreamClient.initDone) {
      yield take(streamActions.streamInitFinished);
    }
    let allFetched = false;
    let history;
    if (filter === 'hashtag') {
      history = yield StreamClient.fetchHashtag(query, hashtag);
    } else if (filter === 'user' || filter === 'local') {
      history = yield StreamClient.fetchMyFeed(query, peerID, filter);
    } else {
      history = yield StreamClient.fetchStream(query);
    }
    let results = history.results;
    if (results.length > 0) {
      allFetched = results.length < STREAM_FEED_PAGE_SIZE;

      const { repost, ...data } = parseFeedData(results);
      if (appending) {
        yield put({ type: streamActions.appendStream, payload: {filter, sort, ...data, allFetched} });
      } else {
        yield put({ type: streamActions.prependStream, payload: {filter, sort, ...data, allFetched} });
      }
      if (repost.length > 0) {
        yield put({ type: streamActions.fetchFeedItems, payload: repost });
      }
    } else {
      yield put({ type: streamActions.finishedFetchRequest })
    }
  } catch (err) {
    yield put({ type: streamActions.finishedFetchRequest })
    console.log('fetch stream saga failure', err);
  }
}

export function* createPost(action) {
  const { reference, data, success, failure } = action.payload;
  try {
    const item = yield StreamClient.createPost(reference, data);
    yield put({ type: streamActions.fetchFeedItems, payload: [item.id] });
    yield put({ type: streamActions.addToRecent, payload: [item.id] });
    if (success) {
      success(item);
    }
    yield put({ type: appstateActions.showToast, payload: { type: 'postCreation', message: item.id } })
    yield delay(3000);
    yield put({ type: appstateActions.hideToast });
  } catch(err) {
    if (failure) {
      failure()
    }
  }
}

export function* likePost(action) {
  const { targetPeerID, activityId, signature } = action.payload;
  try {
    const peerID = yield select(getUserPeerID);
    const activityInfo = (yield select(getActivity))(activityId);
    const status = get(activityInfo, 'object.data.post.status', '');
    yield put({ type: streamActions.updateReaction, payload: { type: 'like', activityId, reaction: signature } });
    yield StreamClient.likePost(activityId, signature);
    if (peerID !== targetPeerID.id) {
      yield put({
        type: streamActions.sendSocialNotification,
        payload: { peerID: targetPeerID.id, verb: 'like', content: { activityId, primaryText: status } }
      });
    }
    yield put({ type: streamActions.fetchFeedItems, payload: [activityId] });
  } catch {
  }
}

export function* commentPost(action) {
  const { targetPeerID, activityId, comment, onSuccess } = action.payload;
  try {
    yield StreamClient.commentPost(activityId, comment);
    const peerID = yield select(getUserPeerID);
    if (targetPeerID !== peerID) {
      yield put({
        type: streamActions.sendSocialNotification,
        payload: { peerID: targetPeerID, verb: 'comment', content: { activityId, secondaryText: comment } }
      });
    }
    yield put({ type: streamActions.fetchFeedItems, payload: [activityId] });
    if (onSuccess) {
      onSuccess();
    }
  } catch {
  }
}

export function* repostPost(action) {
  try {
    const { reference, activityId, post, success } = action.payload;
    const { peerID: targetPeerID } = convertFromReference(reference);
    const userPeerID = yield select(getUserPeerID);
    const item = yield StreamClient.repostPost(activityId, reference, post);
    const withoutComment = post.post.status === '';
    const originalActivifyInfo = (yield select(getActivity))(activityId);
    const status = get(originalActivifyInfo, 'object.data.post.status', '');
    if (targetPeerID !== userPeerID) {
      yield put({
        type: streamActions.sendSocialNotification,
        payload: {
          peerID: targetPeerID,
          verb: 'repost',
          content: {
            activityId: withoutComment ? activityId : item.id,
            originalActivityId: withoutComment ? undefined : activityId,
            primaryText: withoutComment ? status : undefined,
            secondaryText: withoutComment ? undefined : post.post.status,
          }
        }
      });
    }
    yield put({ type: streamActions.fetchFeedItems, payload: [item.id] });
    yield put({ type: streamActions.addToRecent, payload: [item.id] });
    yield put({ type: streamActions.fetchFeedItems, payload: [activityId] });
    if (success) {
      success();
    }
  } catch {
  }
}

export function* updateUser(action) {
  try {
    const { profile } = action.payload;
    yield StreamClient.updateUser(profile);
  } catch {
  }
}

function* postFollowUnfollow(peerID, verb) {
  yield put({ type: streamActions.resetFeedList, payload: 'user' });
  const userSort = yield select(getUserSort);
  const profile = yield select(getMyProfile);
  yield put({
    type: streamActions.fetchStream,
    payload: {
      filter: 'user',
      sort: userSort,
      appending: false,
      peerID: profile.peerID
    }
  })
  yield put({
    type: streamActions.sendSocialNotification,
    payload: { peerID, verb },
  });
}

export function* followUser(action) {
  try {
    const { payload: peerID } = action;
    yield StreamClient.followUser(peerID);
    yield call(postFollowUnfollow, peerID, 'follow');
  } catch {

  }
}

export function* unfollowUser(action) {
  try {
    const { payload: peerID } = action;
    yield StreamClient.unfollowUser(peerID)
    yield call(postFollowUnfollow, peerID, 'unfollow');
  } catch {
  }
}

export function* deleteComment(action) {
  try {
    const { reactionId, onSuccess } = action.payload;
    yield StreamClient.deleteComment(reactionId);
    if (onSuccess) {
      onSuccess(reactionId);
    }
  } catch {
  }
}

export function* deleteRepost(action) {
  try {
    const { payload: { activityId, reactionId, originalActivityId, success } } = action;
    yield StreamClient.deleteRepost(activityId, reactionId);
    yield put({ type: streamActions.fetchFeedItems, payload: [originalActivityId] });
    if (success) {
      success();
    }
  } catch {
  }
}

export function* unlikePost(action) {
  const { payload: { reactionId, activityId } } = action;
  try {
    yield StreamClient.unlikePost(activityId, reactionId);
    yield put({ type: streamActions.updateReaction, payload: { type: 'unlike', activityId, reactionId } });
    yield put({ type: streamActions.fetchFeedItems, payload: [activityId] });
  } catch (err) {

  }
}

export function* deletePost(action) {
  try {
    const { payload: { reference, reactionId, activityId } } = action;
    yield StreamClient.deletePost(reference, activityId, reactionId);
  }catch (err) {

  }
}

export function* removePost(action) {
  try {
    const { payload: { reference, reactionId, activityId } } = action;
    yield StreamClient.removePost(reference, activityId, reactionId);
  }catch (err) {

  }
}

export function* fetchFeedItems(action) {
  const { payload: feedList } = action;
  try {
    if (feedList.length > 0) {
      const { results: feeds = [] } = yield StreamClient.getActivities(feedList);
      const payload = feeds.reduce((prev, current) => {
        const { id } = current;
        return {
          ...prev,
          [id]: { ...current, updated: new moment() },
        }
      }, { });
      yield put({ type: streamActions.setFeedItems, payload });
    }
  } catch {

  }
}

function* subscribeStream(feedGroup, id, readOnlyToken) {
  return eventChannel((emitter) => {
    const subscription = StreamClient.subscribe(feedGroup, id, readOnlyToken, (data) => {
      return emitter({ type: streamActions.updateReceived, payload: data });
    });
    return () => {};
  });
}

function* subscribeNotificationStream(readOnlyToken) {
  const peerID = StreamClient.user.id;
  return eventChannel((emitter) => {
    const subscription = StreamClient.subscribeNotification(readOnlyToken, (data) => {
      return emitter({ type: streamActions.notificationUpdateReceived, payload: data });
    });
    return () => {};
  });
}

function* streamSocketConnect() {
  try {
    const readOnlyToken = yield StreamClient.getReadonlyToken('user', 'all');
    const channel = yield call(subscribeStream, 'user', 'all', readOnlyToken);
    while (true) {
      const action = yield take(channel);
      yield put(action);
    }
  } catch (err) {
  }
}

function* streamNotificationConnect() {
  try {
    const readOnlyToken = yield StreamClient.getReadonlyToken('notifications', StreamClient.user.id);
    const channel = yield call(subscribeNotificationStream, readOnlyToken);
    while (true) {
      const action = yield take(channel);
      yield put(action);
    }
  } catch (err) {

  }
}

function * streamUpdate(action) {
  const { payload: { deleted = [], new: newFeeds = [], feed } } = action;
  if (feed === 'user:all') {
    if (deleted.length > 0) {
      yield put({ type: streamActions.deleteWebsocketUpdate, payload: deleted });
    }
    if (newFeeds.length > 0) {
      const followings = yield select(getFollowings);
      const result = newFeeds.reduce((prev, feed) => {
        const { id: activityId, actor } = feed;
        const result = {...prev};
        if (followings.find(id => id === actor)) {
          result.userFeeds = [...result.userFeeds, activityId];
        }
        result.globalFeeds = [...result.globalFeeds, activityId];
        result.activities = [...result.activites, activityId];
        return result;
      }, { userFeeds: [], globalFeeds: [], activites: [] });
      yield put({ type: streamActions.fetchFeedItems, payload: result.activities });
      yield put({ type: streamActions.addWebsocketUpdate, payload: result });
    }
  }
}

function * notificationUpdate(action) {
  let { payload: { new: newNotifications } } = action;
  newNotifications = newNotifications.filter(n => n.verb !== 'unfollow');
  if (newNotifications.length > 0) {
    const activityIds = newNotifications.reduce((prev, activity) => {
      const activityId = activity.id

      let originalActivityId = activityId
      const message = JSON.parse(activity.message);
      if (message) {
        originalActivityId = message.activityId;
      } else {
        // '{activityId=01f73bc2-5365-11ec-acd5-0ac74274a1c1, secondaryText=Happy a new day}'
        let str = activity.message;
        originalActivityId = str.substring(str.indexOf("=") + 1, str.lastIndexOf(","));
      }

      return originalActivityId ? [...prev, activityId, originalActivityId] : [...prev, activityId];
    }, []);
    yield put({ type: streamActions.fetchFeedItems, payload: activityIds });
    const parsedData = newNotifications.map(activity => ({
      activities: [activity],
      is_read: false,
      is_seen: false,
      activity_count: 1,
      actor_count: 1,
      created_at: activity.time,
      updated: activity.time,
      group: activity.group,
      verb: activity.verb
    }));
    yield put({ type: streamActions.addNotificationUpdate, payload: parsedData });
  }
}

function * sendSocialNotification(action) {
  try {
    const { peerID, type, verb, content } = action.payload;
    yield StreamClient.sendNotification('notifications', type, verb, peerID, content);
  } catch {
  }
}

function * sendNotification(action) {
  try {
    const { peerID, type, verb, content } = action.payload;
    yield StreamClient.sendNotification('normal_notification', type, verb, peerID, content)
  } catch {
  }
}

function * fetchSocialNotifications(action) {
  try {
    const offset = yield select(getNotificationCount);
    const { appending } = action.payload;
    let query = {};
    if (appending) {
      const lastActivityId = yield select(getLastNotificatyionId);
      query = {
        id_lt: lastActivityId ? lastActivityId : undefined,
        offset: lastActivityId ? undefined : offset,
      };
    } else {
      const firstActivityId = yield select(getFirstNotificatyionId);
      query = {
        id_gt: firstActivityId ? firstActivityId : undefined,
        offset: firstActivityId ? undefined : 0,
      };
    }
    let { results: notifications = []} = yield StreamClient.fetchNotification(query);
    notifications = notifications.filter(n => n.verb !== 'unfollow');
    const activityIds = notifications.reduce((prev, notification) => {
      const activity = get(notification, 'activities[0]');

      try {
        const activityId = activity.id

        let originalActivityId = activityId
        const message = JSON.parse(activity.message);
        if (message) {
          originalActivityId = message.activityId;
        } else {
          // '{activityId=01f73bc2-5365-11ec-acd5-0ac74274a1c1, secondaryText=Happy a new day}'
          let str = activity.message;
          originalActivityId = str.substring(str.indexOf("=") + 1, str.lastIndexOf(","));
        }
        return originalActivityId ? [...prev, activityId, originalActivityId] : [...prev, activityId];
      } catch (error) {
        return [...prev]
      }

    }, []);
    yield put({ type: streamActions.fetchFeedItems, payload: activityIds });
    if (appending) {
      yield put({ type: streamActions.appendSocialNotification, payload: { notifications } });
    } else {
      yield put({ type: streamActions.prependSocialNotication, payload: { notifications } });
    }
  } catch (err) {
  }
}

function * markAllAsSeen(action) {
  try {
    const notifications = yield select(getAllNotifications);
    StreamClient.markNotificationAsSeen(notifications.map(notification => notification.id));
  } catch (err) {

  }
}

function * updateSort({ payload }) {
  yield put({ type: streamActions.fetchStream, payload });
}

function * streamInitFinished() {
  yield put({ type: streamActions.fetchSocialNotifications, payload: { appending: false } });
}

export default function* Stream() {
  yield takeEvery(streamActions.fetchStream, fetchStream);
  yield takeEvery(streamActions.refreshStream, fetchStream);
  yield takeEvery(streamActions.updateSort, updateSort);

  yield takeEvery(streamActions.fetchFeedItems, fetchFeedItems);

  yield takeEvery(streamActions.updateUser, updateUser);
  yield takeEvery(streamActions.followUser, followUser);
  yield takeEvery(streamActions.unfollowUser, unfollowUser);

  yield takeEvery(streamActions.createPost, createPost);
  yield takeEvery(streamActions.likePost, likePost);
  yield takeEvery(streamActions.commentPost, commentPost);
  yield takeEvery(streamActions.repostPost, repostPost);

  yield takeEvery(streamActions.deleteComment, deleteComment);
  yield takeEvery(streamActions.deletePost, deletePost);
  yield takeEvery(streamActions.removePost, removePost);
  yield takeEvery(streamActions.deleteRepost, deleteRepost);
  yield takeEvery(streamActions.unlikePost, unlikePost);

  yield takeEvery(streamActions.streamInitFinished, streamInitFinished);
  yield takeEvery(streamActions.streamInitFinished, streamSocketConnect);
  yield takeEvery(streamActions.streamInitFinished, streamNotificationConnect);

  yield takeEvery(streamActions.updateReceived, streamUpdate);
  yield takeEvery(streamActions.notificationUpdateReceived, notificationUpdate);
  yield takeEvery(streamActions.sendSocialNotification, sendSocialNotification);
  yield takeEvery(streamActions.sendNotification, sendNotification);

  yield takeEvery(streamActions.fetchSocialNotifications, fetchSocialNotifications);

  yield takeEvery(streamActions.markAllAsSeen, markAllAsSeen);
};
