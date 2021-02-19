import { gatewayAPI } from './const';
import { serverConfig } from '../utils/server';

// Fetch notifications
export const fetchNotifications = (username, password, limit, offsetId) => {
  const apiURL = `${gatewayAPI}/ob/notifications?limit=${limit}&offsetId=${offsetId}`;
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()));
};

// Mark a notification as 'read'
export const markAsRead = (username, password, notifId) => {
  const apiURL = `${gatewayAPI}/ob/marknotificationasread/${notifId}`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()));
};

// Mark all notifications as 'read'
export const markAsReadAll = (username, password) => {
  const apiURL = `${gatewayAPI}/ob/marknotificationsasread`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()));
};

// Delete a notification
export const deleteNotification = (username, password, notifId) => {
  const apiURL = `${gatewayAPI}/ob/notifications/notifId`;
  const headers = {
    method: 'DELETE',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(
    apiURL,
    headers,
  ).then(response => (response.json()));
};
