"use strict";
import mitt from 'mitt'
import api from "@/api";
import TIM from './TUICore/tim';

class MyChatSDK {
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

  getMessageList() {
    // TODO:
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
    // TODO:
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
    api.getConversationList()
    // TODO:
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

const sdkInstant = mitt(new MyChatSDK());
export default sdkInstant;