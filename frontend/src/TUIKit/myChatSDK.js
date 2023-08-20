"use strict";
import mitt from 'mitt'
import moment from 'moment';
import api from "@/api";

class MyChatSDK {
  emitter = mitt();

  on(type, handler, obj) {
    this.emitter.on(type, handler.bind(obj));
  }

  off(type, handler) {
    this.emitter.off(type, handler);
  }

  emit(type, event) {
    this.emitter.emit(type, event)
  }

  sendMessage() {
    // TODO:
  }
  createFaceMessage() {}
  createImageMessage() {}
  createVideoMessage() {}
  createFileMessage() {}
  createCustomMessage() {}
  createLocationMessage() {}
  createForwardMessage() {}
  sendMessageReadReceipt() {}
  getMessageReadReceiptList() {}

  getMessageList(params) {
    return api.getConversationMessage({peerID: params.conversationID, limit: params.count, offsetID: params.nextReqMessageID}).then((list) => {
      list = list.map(item => (
        {
          ID: item.messageID,
          type: window.TIM.TYPES.MSG_TEXT,
          conversationID: item.peerID,
          conversationType: window.TIM.TYPES.CONV_C2C,
          to: item.outgoing ? item.peerID : '',
          from: !item.outgoing ? item.peerID : '',
          time: moment(item.timestamp).unix(),
          payload: { text: item.message },
        }
      ));

      return {data: { messageList: list }};
    });
  }
  createTextMessage() {
    // TODO:
  }
  createTextAtMessage() {}
  createMergerMessage() {}
  revokeMessage() {}
  resendMessage() {}
  deleteMessage() {}
  modifyMessage() {}
  findMessage() {}

  getUserProfile() {
    // TODO:
  }
  getMyProfile() {
    return api.getMyProfile().then((profile) => (
      {
        data: {
          userID: profile.peerID,
          nick: profile.name,
          location: profile.location,
          selfSignature: profile.about,
          avatar: window['app']?.getServerUrl(`ob/image/${profile?.avatarHashes?.small}`),
        },
      }
    ));
  }
  updateMyProfile() {
    // TODO:
  }

  getFriendList() {}
  checkFriend() {}
  
  setMessageRead() {}

  getUserStatus() {}
  subscribeUserStatus() {}
  unsubscribeUserStatus() {}

  getConversationList() {
    return api.getConversationList().then((list => {
      list = list.map(item => ({
        conversationID: item.conversationID,
        type: window.TIM.TYPES.CONV_C2C,
        unreadCount: item.unread,
        lastMessage: {
          messageForShow: item.lastMessage,
        },
        userProfile: {},
      }));
      return { data: {conversationList: list} };
    }))
  }
  getConversationProfile() {
    // TODO:
  }
  deleteConversation() {}
  pinConversation() {}
  setMessageRemindType() {}

  createGroup() {}
  getGroupMessageReadMemberList() {}
  getGroupProfile() {}
  getGroupMemberProfile() {}
  handleGroupApplication() {}
  getGroupList() {}
  dismissGroup() {}
  updateGroupProfile() {}
  joinGroup() {}
  quitGroup() {}
  searchGroupByID() {}
  getGroupOnlineMemberCount() {}
  changeGroupOwner() {}
  initGroupAttributes() {}
  setGroupAttributes() {}
  deleteGroupAttributes() {}
  getGroupAttributes() {}
  getGroupMemberList() {}
  addGroupMember() {}
  deleteGroupMember() {}
  setGroupMemberMuteTime() {}
  setGroupMemberRole() {}
  setGroupMemberNameCard() {}
}

class ProfileHandler {
  constructor(e) {
    this._n = "ProfileHandler";
    this.TAG = "profile";
  }

  setExpirationTime(e) {
    this.expirationTime = e;
  }

  getUserProfile(userIDList) {}

  getMyProfile() {}
}

class C2CModule {
  setMessageRead(options) {}

  deleteConversation(options) {}

  pinConversation() {}

  setMessageRemindType() {}

  getConversationProfile() {}

  getConversationList() {}
}

class GroupHandler {}

/* Events
this.TUICore.TIM.EVENT.MESSAGE_RECEIVED
this.TUICore.TIM.EVENT.MESSAGE_MODIFIED
this.TUICore.TIM.EVENT.MESSAGE_REVOKED
this.TUICore.TIM.EVENT.MESSAGE_READ_BY_PEER

this.TUICore.TIM.EVENT.GROUP_LIST_UPDATED

this.TUICore.TIM.EVENT.MESSAGE_READ_RECEIPT_RECEIVED

this.TUICore.TIM.EVENT.GROUP_ATTRIBUTES_UPDATED
this.TUICore.TIM.EVENT.CONVERSATION_LIST_UPDATED
this.TUICore.TIM.EVENT.FRIEND_LIST_UPDATED
this.TUICore.TIM.EVENT.USER_STATUS_UPDATED
this.TUICore.TIM.EVENT.NET_STATE_CHANGE
this.TUICore.TIM.EVENT.PROFILE_UPDATED
*/

/* TODO
this.TUICore.TIM.EVENT.MESSAGE_RECEIVED
this.TUICore.TIM.EVENT.CONVERSATION_LIST_UPDATED
*/

const sdkInstant = new MyChatSDK();
export default sdkInstant;