import { myGet } from './api';

export default {
  getConversationList(params = {}) {
    return myGet(window.app.getServerUrl('ob/chatconversations'), params);
  },

  getMyProfile(params = {}) {
    return myGet(window.app.getServerUrl('ob/profile'), params);
  },

  getConversationMessage(params) {
    return myGet(window.app.getServerUrl(`ob/chatmessages/${params.peerID}`, params));
  },
};
