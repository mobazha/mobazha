import { gatewayAPI } from './const';
import { serverConfig } from '../utils/server';

// Estimate the cost of a listing
export const getEstimation = (username, password, checkoutData) => {
  const apiURL = `${gatewayAPI}/ob/estimatetotal`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(checkoutData),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch(err => ({ success: false, err }));
};

// Checkout Breakdown
export const getCheckoutBreakdown = (username, password, checkoutData) => {
  const apiURL = `${gatewayAPI}/ob/checkoutbreakdown`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(checkoutData),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch(err => ({ success: false, err }));
};

// Create a purchase order
export const purchaseItem = (username, password, checkoutData) => {
  const apiURL = `${gatewayAPI}/ob/purchase`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(checkoutData),
  };
  return fetch(apiURL, headers).then(response => response.json());
};
