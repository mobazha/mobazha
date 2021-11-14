import * as _ from 'lodash';
import uuidv4 from 'uuid/v4';

import { gatewayAPI, searchAPI } from './const';
import { serverConfig } from '../utils/server';

export const postFeed = (status, images) => {
  const apiURL = `${gatewayAPI}/ob/post`;
  const uuid = uuidv4();
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
    body: JSON.stringify({
      status,
      images,
      tags: [],
      channels: [],
      reference: '',
      postType: 'POST',
      longForm: '',
      slug: uuid,
    }),
  };
  return fetch(apiURL, headers).then(response => response.json());
};

export const fetchFeedItem = (slug, peerID = '') => {
  let apiURL = `${gatewayAPI}/ob/post/${slug}?`;
  if (peerID !== '') {
    apiURL = `${searchAPI}/api/posts/${peerID}/${slug}?type=post&type=repost&includes=likes&includes=reposts&includes=comments`;
  }
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log(err);
      return [];
    });
};

export const deleteFeedItem = (slug) => {
  const apiURL = `${gatewayAPI}/ob/post/${slug}`;
  const headers = {
    method: 'DELETE',
    headers: serverConfig.getAuthHeader(),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log(err);
      return [];
    });
};

export const likePost = (peer_id, post_peer_id, slug, signature = '*') => {
  const apiURL = `${searchAPI}/api/posts/like`;
  const headers = {
    method: 'POST',
    body: JSON.stringify({
      peer_id, post_peer_id, slug, signature,
    }),
  };
  return fetch(apiURL, headers).then(response => response.json());
};

export const signMessage = (reference) => {
  const apiURL = `${gatewayAPI}/ob/signmessage`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
    body: JSON.stringify({
      content: reference,
    }),
  };
  return fetch(apiURL, headers).then(response => response.json());
};

export const commentFeed = (reference, comment) => {
  const apiURL = `${gatewayAPI}/ob/post`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
    body: JSON.stringify({
      status: comment,
      longForm: '',
      images: [],
      tags: [],
      channel: [],
      postType: 'COMMENT',
      reference,
    }),
  };
  return fetch(apiURL, headers).then(response => response.json());
};

export const repostFeed = (reference, comment) => {
  const apiURL = `${gatewayAPI}/ob/post`;
  const uuid = uuidv4();
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
    body: JSON.stringify({
      status: comment,
      longForm: '',
      images: [],
      tags: [],
      channel: [],
      postType: 'REPOST',
      slug: uuid,
      reference,
    }),
  };
  return fetch(apiURL, headers).then(response => response.json());
};
