import $ from 'jquery';

export default {
  getConversationList(params = {}) {
    return $.get(window.app.getServerUrl('ob/chatconversations'), params);
  },

  getMyProfile(params = {}) {
    return $.get(window.app.getServerUrl('ob/profile'), params);
  },

  getConversationMessage(params) {
    return $.get(window.app.getServerUrl(`ob/chatmessages/${params.peerID}`, params));
  },
};
