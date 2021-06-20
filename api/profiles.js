import { stringify } from 'query-string';

import { mbzEthGatewayAPI } from './const';

// Fetch batch profiles
export const getProfiles = (peerIDList, async = false) => {
  const timestamp = Date.now();
  const searchQuery = { async };
  const queryString = stringify(searchQuery);
  const apiURL = `${mbzEthGatewayAPI}/ob/fetchprofiles?${queryString}&${timestamp}`;

  const headers = {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(peerIDList),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch(err => err);
};
