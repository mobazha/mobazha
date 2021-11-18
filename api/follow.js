import { isEmpty } from 'lodash';

import { gatewayAPI, searchAPI } from './const';
import { makeRequest } from './common';
import { serverConfig } from '../utils/server';

// Fetch the followers of a user/store
export const getFollowers = (peerID = '') => {
  const timestamp = Date.now();
  const apiURL = `${searchAPI}/api/profile/followers?peerId=${peerID}&${timestamp}&pageSize=10000`;
  return makeRequest(apiURL, isEmpty(peerID)).then(
    result => result.followers.map(item => item.peerId)
  );
};

// Fetch the following list of a user/store
export const getFollowings = (peerID = '') => {
  const timestamp = Date.now();
  const apiURL = `${searchAPI}/api/profile/following?peerId=${peerID}&${timestamp}&pageSize=10000`;
  return makeRequest(apiURL, isEmpty(peerID)).then(
    result => result.following.map(item => item.peerId)
  );
};

// Check if the user is following the store
export const getFollowingsFromLocal = () => {
  const apiURL = `${gatewayAPI}/ob/following`;
  return makeRequest(apiURL, true);
};

// Follow a store
export const followPeer = (peerID) => {
  const apiURL = `${gatewayAPI}/ob/follow`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
    body: JSON.stringify({
      id: peerID,
    }),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch(() => ({ isFollowing: false }));
};

// Unfollow a store
export const unfollowPeer = (peerID) => {
  const apiURL = `${gatewayAPI}/ob/unfollow`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
    body: JSON.stringify({
      id: peerID,
    }),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch(() => ({ isFollowing: false }));
};
