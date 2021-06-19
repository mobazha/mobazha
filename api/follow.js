import { isEmpty } from 'lodash';

import { obGatewayAPI, gatewayAPI, searchAPI } from './const';
import { makeRequest } from './common';
import { serverConfig } from '../utils/server';

// Fetch the followers of a user/store
export const getFollowers = (peerID = '') => {
  const timestamp = Date.now();
  const apiURL = `${gatewayAPI}/ob/followers/${peerID}?usecache=true&${timestamp}`;
  return makeRequest(apiURL, true);
};

// Fetch the following list of a user/store
export const getFollowings = (peerID = '') => {
  const timestamp = Date.now();
  const apiURL = `${gatewayAPI}/ob/following/${peerID}?usecache=true&${timestamp}`;
  return makeRequest(apiURL, true);
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
