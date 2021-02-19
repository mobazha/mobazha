import { serverConfig } from '../utils/server';
import { gatewayAPI } from './const';

// Fetch history of purchases
export const getPurchases = (username, password, limit = '', offsetId = '') => {
  const apiURL = `${gatewayAPI}/ob/purchases?limit=${limit}&offsetId=${offsetId}`;
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => response.json())
    .catch(err => (err));
};

// Fetch history of sales
export const getSales = (username, password, limit = '', offsetId = '') => {
  const apiURL = `${gatewayAPI}/ob/sales?limit=${limit}&offsetId=${offsetId}`;
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Cancel an order
export const cancelOrder = (username, password, orderId) => {
  const apiURL = `${gatewayAPI}/ob/ordercancel`;
  const body = {
    orderId,
  };
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Fund an order
export const fundOrder = (username, password, payload) => {
  const {
    coin, address, amount, orderId, memo,
  } = payload;
  const apiURL = `${gatewayAPI}/ob/orderspend`;
  const body = {
    currencyCode: coin,
    orderID: orderId,
    address,
    amount,
    feeLevel: 'SUPER_ECONOMIC',
    memo,
    requireAssociateOrder: true,
  };
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Refund an order
export const refundOrder = (username, password, orderId) => {
  const apiURL = `${gatewayAPI}/ob/refund`;
  const body = {
    orderId,
  };
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Fulfill an order
export const fulfillOrder = (username, password, fulfillObj) => {
  const apiURL = `${gatewayAPI}/ob/orderfulfillment`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(fulfillObj),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Complete an order
export const completeOrder = (username, password, payload) => {
  const apiURL = `${gatewayAPI}/ob/ordercompletion`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(payload),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Open a dispute
export const openDispute = (username, password, orderId, claim) => {
  const apiURL = `${gatewayAPI}/ob/opendispute`;
  const body = {
    orderId,
    claim,
  };
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Accept a dispute decision from the moderator
export const acceptDispute = (username, password, orderId) => {
  const apiURL = `${gatewayAPI}/ob/releasefunds`;
  const body = {
    orderId,
  };
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Accept a dispute decision from the moderator
export const claimPayment = (username, password, orderId) => {
  const apiURL = `${gatewayAPI}/ob/releaseescrow`;
  const body = {
    orderId,
  };
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Accept a dispute decision from the moderator
export const confirmOrder = (username, password, payload) => {
  const apiURL = `${gatewayAPI}/ob/orderconfirmation`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(payload),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()));
};

// Fetch order details
export const getOrderDetails = (username, password, orderId) => {
  const apiURL = `${gatewayAPI}/ob/order/${orderId}`;
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Fetch order details
export const scanOfflineMessages = (username, password) => {
  const apiURL = `${gatewayAPI}/ob/scanofflinemessages`;
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};

// Fetch order details
export const resendOrderMessage = (username, password, orderId, messageType) => {
  const apiURL = `${gatewayAPI}/ob/resendordermessage`;
  const body = { orderId, messageType };
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body: JSON.stringify(body),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()))
    .catch(err => (err));
};
