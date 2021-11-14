import { isEmpty } from 'lodash';

import { gatewayAPI, searchAPI } from './const';
import { serverConfig } from '../utils/server';
import { makeFetch } from './common';

export const getProfile = (peerID = '') => {
// Fetch the user or store profile

  let apiURL = '';
  const timestamp = Date.now();

  if (isEmpty(peerID)) {
    apiURL = `${gatewayAPI}/ob/profile?async=true`;
  } else {
    apiURL = `${searchAPI}/api/profile?peerId=${peerID}&${timestamp}`;
  }
  const headers = {
    method: 'GET',
    headers: isEmpty(peerID) ? serverConfig.getAuthHeader() : {},
  };
  return fetch(
    apiURL,
    headers,
  )
    .then((response) => {
      if (response.status === 404) {
        return null;
      } else {
        return response.json();
      }
    });
};

// Set the user's profile
export const setProfile = profile => makeFetch({
  url: `${gatewayAPI}/ob/profile`,
  method: 'PUT',
  body: JSON.stringify({ ...profile, vendor: true }),
});

// Set the user's accepted coins
export const setCoins = coins => makeFetch({
  url: `${gatewayAPI}/ob/bulkupdatecurrency`,
  method: 'POST',
  body: JSON.stringify({ currencies: coins }),
});

export const reportProfile = (peerID, reason, slug = '', report_type = 'node') => {
  const headers = {
    method: 'POST',
    body: JSON.stringify({ peerID, reason, slug, report_type }),
  };
  return fetch(`${searchAPI}/api/reports`, headers);
};
