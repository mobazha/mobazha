import * as _ from 'lodash';
import { mbzEthGatewayAPI, gatewayAPI, searchAPI } from './const';
import { serverConfig } from '../utils/server';

// Fetch Mobazha verified moderators
export const getModerators = () => {
  const timestamp = Date.now();
  const url = `${searchAPI}/api/moderator/verified?${timestamp}`;
  const headers = {
    method: 'GET',
  };
  return fetch(url, headers)
    .then(response => (response.json()))
    .catch((err) => {
      console.log('Error while fetching Moderators List', err);
      return [];
    });
};

// Add moderator to the user's 'approved moderator' list
export const setModerator = (username, password, body) => {
  const url = `${gatewayAPI}/ob/moderator`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(url, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log('Error whilet setting Moderator Info', err);
    });
};

// Fetch data about the moderator
export const getModerator = (username, password, peerID) => {
  const timestamp = Date.now();
  let url = '';
  if (peerID) { url = `${mbzEthGatewayAPI}/ob/moderator/${peerID}?${timestamp}`; } else { url = `${gatewayAPI}/ob/moderator`; }
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(url, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log('Error while getting Moderator Info', err);
    });
};

// Update moderator data
export const updateModerator = (username, password, body) => {
  const url = `${gatewayAPI}/ob/moderator`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(url, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log('Error while update Moderator', err);
      return {};
    });
};

// Remove moderator
export const unsetModerator = (username, password, peerID) => {
  const url = `${gatewayAPI}/ob/moderator/${peerID}`;
  const headers = {
    method: 'DELETE',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(url, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log('Error while unset Moderator', err);
      return {};
    });
};
