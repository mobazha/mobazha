import moment from 'moment';
import queryString from 'querystring';
import Crypto from 'crypto-js';
import { hasIn, get, isEmpty } from 'lodash';
import AsyncStorage from '@react-native-community/async-storage';
import Config from 'react-native-config';
import Reactotron from 'reactotron-react-native';

import { streamTokenAPI } from './const';
import { signMessage } from './signing';
import { getHashTags } from '../utils/hashtags';

const Base64 = require('base-64');

const { HMAC_SECRET } = Config;

const getPubKey = () => signMessage('pubkey')
  .then(response => response.pubkey);

const signAuthChallenge = (userId, challenge) => signMessage(`${userId}${challenge}`)
  .then(response => response.signature);

const getAuthChallenge = (peerId, pubkey) => fetch(
  `${streamTokenAPI}authChallenge?pubkey=${pubkey}&userId=${peerId}`,
  {
    method: 'POST',
  },
).then(response => response.json()).then(response => response.challenge);

export class StreamAuth {
  constructor(userId) {
    this.userId = userId;
  }

  initStreamClient = async (ignoreCache) => {
    try {
      if (ignoreCache) {
        throw new Error('Ignoring cache and refreshing challenge header...');
      }

      const savedChallengeHeader = await AsyncStorage.getItem('challengeHeader');
      if (!isEmpty(savedChallengeHeader)) {
        this.challengeHeader = savedChallengeHeader;
        Reactotron.log('restored saved challenge header', this.challengeHeader);
      } else {
        throw new Error('saved challenge header is empty');
      }
    } catch (_) {
      try {
        const pubkey = await getPubKey(this.userId);
        const challenge = await getAuthChallenge(this.userId, pubkey);
        const signedChallenge = await signAuthChallenge(this.userId, challenge);
        this.challengeHeader = Base64.encode(JSON.stringify({
          userId: this.userId, pubkey, challenge: signedChallenge,
        }));
        AsyncStorage.setItem('challengeHeader', this.challengeHeader);
      } catch (err) {
        console.log(err);
      }
    }
  }

  getAuthToken() {
    const unixTimestamp = Math.floor(moment().valueOf() / 1000);
    const key = Crypto.enc.Utf8.parse(HMAC_SECRET);
    const signingString = Crypto.enc.Utf8.parse(`${this.userId}${unixTimestamp}`);
    const data = Crypto.enc.Hex.stringify(Crypto.HmacSHA256(signingString, key));
    const objectStr = JSON.stringify({ timestamp: unixTimestamp, data });
    return Base64.encode(objectStr);
  }

  getAuthHeader() {
    const authToken = this.getAuthToken();
    return {
      'X-Signature': authToken,
      'Content-type': 'application/json',
      Authorization: this.challengeHeader,
    };
  }

  processRequest(apiURL, headers) {
    return fetch(apiURL, headers)
      .then(response => response.json())
      .then((response) => {
        if (hasIn(response, 'message')) {
          if (response.message === 'Token time out') {
            Reactotron.log('re-init stream client from processRequest', new Date());
            this.initStreamClient(true);
          }
          throw Error(response.message);
        } else if (response.error) {
          throw response.error;
        } else {
          return response;
        }
      });
  }

  getStreamToken = () => {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}getToken`;
    const headers = {
      method: 'GET',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
    };
    return this.processRequest(apiURL, headers);
  }

  getReadonlyToken(feed_group, feed_id) {
    const authToken = this.getAuthToken();
    const qs = queryString.stringify({ feed_group, feed_id, user_id: this.userId });
    const apiURL = `${streamTokenAPI}getReadOnlyToken?${qs}`;
    const headers = {
      method: 'GET',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
    };
    return this.processRequest(apiURL, headers);
  }

  followMany(followings) {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}followMany`;
    const headers = {
      method: 'PUT',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify(followings),
    };
    return this.processRequest(apiURL, headers);
  }

  postFeed = (data, slug, foreignId) => {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}postFeed`;
    const { status } = data.post;
    const hashtags = getHashTags(status);
    const headers = {
      method: 'POST',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ data, slug, foreignId, hashtags }),
    };
    return this.processRequest(apiURL, headers);
  }

  repostPost = (payload) => {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}repostPost`;
    const { status } = get(payload, 'post.post.status');
    const hashtags = getHashTags(status);
    const headers = {
      method: 'POST',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ ...payload, hashtags }),
    };
    return this.processRequest(apiURL, headers);
  }

  likePost = (activityId, signature) => {
    const apiURL = `${streamTokenAPI}likePost`;
    const headers = {
      method: 'POST',
      headers: this.getAuthHeader(),
      body: JSON.stringify({ activityId, signature }),
    };
    return this.processRequest(apiURL, headers);
  }

  unlikePost = (activityId, reactionId) => {
    const apiURL = `${streamTokenAPI}unlikePost`;
    const headers = {
      method: 'POST',
      headers: this.getAuthHeader(),
      body: JSON.stringify({ activityId, reactionId }),
    };
    return this.processRequest(apiURL, headers);
  }

  commentPost = (activityId, comment) => {
    const apiURL = `${streamTokenAPI}commentPost`;
    const headers = {
      method: 'POST',
      headers: this.getAuthHeader(),
      body: JSON.stringify({ activityId, comment }),
    };
    return this.processRequest(apiURL, headers);
  }

  deleteComment = (reactionId) => {
    const apiURL = `${streamTokenAPI}deleteComment`;
    const headers = {
      method: 'POST',
      headers: this.getAuthHeader(),
      body: JSON.stringify({ reactionId }),
    };
    return this.processRequest(apiURL, headers);
  }

  deleteRepost = (activityId, reactionId) => {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}deleteRepost`;
    const headers = {
      method: 'POST',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ activityId, reactionId }),
    };
    return this.processRequest(apiURL, headers);
  }

  deletePost = (reference, activityId) => {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}deletePost`;
    const headers = {
      method: 'POST',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ reference, activityId }),
    };
    return this.processRequest(apiURL, headers);
  }

  removePost = (reference, activityId) => {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}removePost`;
    const headers = {
      method: 'POST',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ reference, activityId }),
    };
    return this.processRequest(apiURL, headers);
  }

  createUser(profile) {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}getOrCreateProfile`;
    const headers = {
      method: 'post',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ profile }),
    };
    return this.processRequest(apiURL, headers);
  }

  updateUser(profile) {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}updateProfile`;
    const headers = {
      method: 'put',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ profile }),
    };
    return this.processRequest(apiURL, headers);
  }

  sendNotification(notificationCategory, type, verb, targetPeerID, content) {
    const authToken = this.getAuthToken();
    const apiURL = `${streamTokenAPI}sendNotification`;
    const headers = {
      method: 'post',
      headers: {
        'X-Signature': authToken,
        'Content-type': 'application/json',
        Authorization: this.challengeHeader,
      },
      body: JSON.stringify({ notificationCategory, type, verb, targetUserId: targetPeerID, content }),
    };
    return this.processRequest(apiURL, headers);
  }
}
