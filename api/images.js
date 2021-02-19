import { gatewayAPI } from './const';
import { serverConfig } from '../utils/server';

// Upload an image
export const uploadImage = (username, password, image) => {
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify([image]),
  };
  return fetch(
    `${gatewayAPI}/ob/images`,
    headers,
  ).then(response => (response.json()))
    .catch((err) => {
      console.log(err);
      return [];
    });
};
