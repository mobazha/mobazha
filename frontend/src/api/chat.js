import $ from 'jquery';

export default {
  getConversationList(params = {}) {
    return $.get(window.app.getServerUrl('ob/chatconversations'), params);
  },
};
