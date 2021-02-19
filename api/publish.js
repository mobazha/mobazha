
import { gatewayAPI, searchAPI } from './const';
import { serverConfig } from '../utils/server';

export const publish = (username, password) => {
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    `${gatewayAPI}/ob/publish`,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

export const ingestPeer = (peerID, body) => {
  const headers = {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  };
  return fetch(`${searchAPI}/ipns/${peerID}`, headers)
    .then(response => (response.json()))
    .catch(err => (err));
};

export const resolveIpns = (peerID, username, password) => {
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(`${gatewayAPI}/ob/resolveipns`, headers)
    .then(response => (response.json()))
    .catch(err => err);
};
