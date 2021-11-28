import { get, isEmpty } from 'lodash';
import AsyncStorage from '@react-native-community/async-storage';
import Reactotron from 'reactotron-react-native';
import * as _ from 'lodash';
import Config from 'react-native-config';

import { StreamAuth } from './api/stream';
import { convertFromReference } from './utils/feed';
import { STREAM_FEED_PAGE_SIZE } from './config/stream';

const stream = require('getstream');

const { STREAM_API_KEY, STREAM_APP_ID } = Config;
console.warn('---', JSON.stringify(Config));

class OBStream {
  client;
  feed;
  tokens;
  user;
  readonlyToken;
  socketClient;
  authHolder;

  async initializeStream(profile, followings = [], isFirst = false) {
    const { peerID } = profile;

    let tokens;
    let savedTokens;

    this.authHolder = new StreamAuth(peerID);

    let challengeHeaderCacheWasInvalid = false;

    try {
      Reactotron.log('cache - request for create stream user', new Date());
      await this.authHolder.initStreamClient();
      await this.createUser(profile);
      Reactotron.log('cache - request finished for stream client init', new Date());
    } catch (_) {
      challengeHeaderCacheWasInvalid = true;

      Reactotron.log('no cache - request for create stream user', new Date());
      await this.authHolder.initStreamClient(true);
      await this.createUser(profile);
      Reactotron.log('no cache - request finished for stream client init', new Date());
    }

    try {
      if (challengeHeaderCacheWasInvalid) {
        throw new Error('Challenge Header Cache was invalid, refreshing stream tokens...');
      }

      savedTokens = await AsyncStorage.getItem('streamTokens');
      if (!savedTokens) {
        throw new Error('Saved Token is null')
      }

      tokens = JSON.parse(savedTokens);
      Reactotron.log('restored saved token', savedTokens);
    } catch (_) {
      Reactotron.log('requesting stream token', new Date());

      tokens = await this.authHolder.getStreamToken();
      AsyncStorage.setItem('streamTokens', JSON.stringify(tokens));
      Reactotron.log('request finished for stream token', new Date());
    }

    if (isFirst) {
      Reactotron.log('request for follow many', new Date());
      this.authHolder.followMany(followings);
    }
    this.tokens = tokens;
    const { activityToken, feedToken, reactionsToken, followToken } = tokens;
    this.activityClient = stream.connect(STREAM_API_KEY, activityToken, STREAM_APP_ID, {location: 'us-east'});
    this.feedClient = stream.connect(STREAM_API_KEY, feedToken, STREAM_APP_ID, {location: 'us-east'});
    this.followClient = stream.connect(STREAM_API_KEY, followToken, STREAM_APP_ID, {location: 'us-east'});
    this.followFeed = this.followClient.feed('user', peerID, followToken);
    this.localFeed = this.feedClient.feed('posts', peerID, feedToken);
    this.feed = this.feedClient.feed('user', peerID, feedToken);
    this.globalFeed = this.feedClient.feed('user', 'all', feedToken);
    this.notificationFeed = this.feedClient.feed('notifications', peerID, feedToken);
    this.socketClient = stream.connect(STREAM_API_KEY, null, STREAM_APP_ID, {location: 'us-east'});
    Reactotron.log('connected to stream', new Date());
    this.initDone = true;
  }

  getFeedQuery = (user_id, query, limit = STREAM_FEED_PAGE_SIZE) => ({
    user_id,
    limit,
    enrich: true,
    reactions: { own: true, recent: true, counts: true },
    ...query,
  })

  async fetchHashtag(query, hashtag) {
    try {
      const user_id = this.user.id;
      const feedQuery = this.getFeedQuery(user_id, query);
      if (hashtag) {
        const feed = this.feedClient.feed('tag', hashtag, this.globalToken);
        return await feed.get(feedQuery);
      }
      return { results: [] };
    } catch (err) {
      return { results: [] };
    }
  }

  async fetchStream(query) {
    try {
      const user_id = this.user.id;
      const result = await this.globalFeed.get(this.getFeedQuery(user_id, query));
      return result;
    } catch (err) {
      return { results: [] };
    }
  }

  async fetchNotification(query) {
    try {
      return await this.notificationFeed.get({ limit: 30, ...query });
    } catch {
      return { results: [] };
    }
  }

  async fetchMyFeed(query, peerID = '', filter) {
    try {
      const user_id = this.user.id;
      const feedQuery = this.getFeedQuery(user_id, query);
      if (isEmpty(peerID)) {
        if (filter === 'local') {
          return await this.localFeed.get(feedQuery)
        }
        return await this.feed.get(feedQuery);
      } else {
        const feed = this.feedClient.feed(filter === 'local' ? 'posts' : 'user', peerID, this.tokens.globalToken);
        const result = await feed.get(feedQuery);
        const results = result.results.filter((feed) => feed.actor.id === peerID);
        return { ...result, results };
      }
    } catch {
      return { results: [] };
    }
  }

  async createUser(profile) {
    const { user } = await this.authHolder.createUser(profile);
    this.user = user;
  }

  async updateUser(profile) {
    await this.authHolder.updateUser(profile);
  }

  async followUser(peerID) {
    await this.followFeed.follow('user', peerID);
  }

  async unfollowUser(peerID) {
    this.followFeed.unfollow('user', peerID);
  }

  // Create a post
  async createPost(reference, data) {
    const { slug } = convertFromReference(reference);
    return await this.authHolder.postFeed(data, slug, reference);
  }

  async deletePost(reference, activityId) {
    this.authHolder.deletePost(reference, activityId);
  }

  async removePost(reference, activityId) {
    this.authHolder.removePost(reference, activityId);
  }

  async likePost(activityId, signature) {
    await this.authHolder.likePost(activityId, signature);
  }

  async unlikePost(activityId, reactionId) {
    await this.authHolder.unlikePost(activityId, reactionId);
  }

  async commentPost(activityId, comment) {
    await this.authHolder.commentPost(activityId, comment);
  }

  async repostPost(activityId, reference, post) {
    const { peerID } = convertFromReference(reference);
    const reactionActivity = await this.authHolder.repostPost({ activityId, reference, post, ownerId: peerID });
    return reactionActivity;
  }

  async deleteRepost(activityId, reactionId) {
    this.authHolder.deleteRepost(activityId, reactionId);
  }

  async deleteComment(reactionId) {
    this.authHolder.deleteComment(reactionId);
  }

  async getActivities(activityIds) {
    try {
      const user_id = this.user.id;
      return await this.activityClient.getActivities({
        user_id,
        ids: activityIds.filter(activityId => !_.isEmpty(activityId)),
        enrich: true,
        reactions: { own: true, recent: true, counts: true }
      });
    } catch {
      return [];
    }
  }

  async getReactions(activity_id, kind) {
    if (!this.feedClient) {
      throw 'STREAM_NOT_INITIALIZED';
    }

    return await this.feedClient.reactions.filter({ activity_id, kind });
  }

  async sendNotification(notificationCategory, type, verb, targetPeerID, content) {
    this.authHolder.sendNotification(notificationCategory, type, verb, targetPeerID, content);
  }

  getReadonlyToken = async (feedGroup, feedId) => {
    const { readonlyToken } = await this.authHolder.getReadonlyToken(feedGroup, feedId);
    return readonlyToken
  }

  async subscribe(feedGroup, feedId, readonlyToken, subscribingFunction) {
    this.socketFeed = this.socketClient.feed(feedGroup, feedId, readonlyToken);
    return this.socketFeed.subscribe(subscribingFunction);
  }

  async subscribeNotification(readonlyToken, subscribingFunction) {
    this.notificaitonSocketFeed = this.socketClient.feed('notifications', StreamClient.user.id, readonlyToken);
    return this.notificaitonSocketFeed.subscribe(subscribingFunction);
  }

  async markNotificationAsSeen(groupIds) {
    this.notificationFeed.get({ mark_seen: groupIds });
  }
}

const StreamClient = new OBStream();

export default StreamClient;
