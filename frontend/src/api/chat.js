import $ from 'jquery';

export default {
  getConversationList(params = {}) {
    return $.get($.app.getServerUrl('ob/chatconversations'), params);
  },
};
