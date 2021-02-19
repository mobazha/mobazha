import { gatewayAPI } from './const';
import { serverConfig } from '../utils/server';

// Fetch chat conversations
export const fetchChats = (username, password) => {
  const apiURL = `${gatewayAPI}/ob/chatconversations`;
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(apiURL, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log(err);
      return [];
    });
};

// Fetch chat messages from a conversation
export const fetchChatDetail = (username, password, peerID, subject) => {
  let apiURL;
  if (peerID) {
    apiURL = `${gatewayAPI}/ob/chatmessages/${peerID}?subject=${subject}`;
  } else {
    apiURL = `${gatewayAPI}/ob/chatmessages?limit=&offsetId=&subject=${subject}`;
  }
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(apiURL, headers).then(response => response.json());
};

// Send a chat message
export const sendChat = (username, password, peerIds, subject = '', message) => {
  let apiURL;
  let peerIdPayload;
  if (typeof peerIds === 'string') {
    apiURL = `${gatewayAPI}/ob/chat`;
    peerIdPayload = { peerId: peerIds };
  } else {
    apiURL = `${gatewayAPI}/ob/groupchat`;
    peerIdPayload = { peerIds };
  }

  const body = JSON.stringify({
    subject,
    message,
    ...peerIdPayload,
  });

  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
    body,
  };
  return fetch(apiURL, headers).then(response => response.json());
};

// Set Chat As Read
export const setChatAsRead = (username, password, peerID, subject) => {
  let apiURL;
  if (peerID) {
    apiURL = `${gatewayAPI}/ob/markchatasread/${peerID}?subject=${subject}`;
  } else {
    apiURL = `${gatewayAPI}/ob/markchatasread?subject=${subject}`;
  }
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(apiURL, headers).then(response => response.json());
};

export const deleteChatConversation = (username, password, peerID) => {
  const apiURL = `${gatewayAPI}/ob/chatconversation/${peerID}`;
  const headers = {
    method: 'DELETE',
    headers: serverConfig.getAuthHeader(username, password),
  };
  return fetch(apiURL, headers).then(response => response.json());
};

