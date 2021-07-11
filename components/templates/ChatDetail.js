import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Keyboard, View, Dimensions, Text, Platform, ActivityIndicator, TouchableWithoutFeedback } from 'react-native';
import { GiftedChat, Send, InputToolbar } from 'react-native-gifted-chat';
import decode from 'unescape';
import * as _ from 'lodash';
import deepEqual from 'deep-equal';
import moment from 'moment';
import { withNavigation } from 'react-navigation';

import AvatarImage from '../atoms/AvatarImage';
import ChatBubble from '../atoms/ChatBubble';
import MessageText from '../atoms/MessageText';
import { serverConfig } from '../../utils/server';

import {
  fetchChats,
  fetchChatDetail,
  sendChat,
  setChatAsRead,
  setChatDetailLoading,
} from '../../reducers/chat';
import { blockNode } from '../../reducers/settings';
import { getDefaultProfileFromPeerId } from '../../utils/profile';
import { convertMsgToGiftChatItem } from '../../utils/chat';
import { websocketHost } from '../../api/const';
import { wrapperStyles } from '../../utils/styles';
import {
  foregroundColor,
  brandColor,
  primaryTextColor,
  mainBorderColor,
  secondaryBackgroundColor,
  warningColor,
  borderColor,
} from '../commonColors';

import {I18n} from '../../langs/I18n';

const { width, height } = Dimensions.get('window');

const styles = {
  placeholderWrapper: {
    position: 'absolute',
    alignItems: 'center',
    justifyContent: 'center',
    height: 150,
    width: 300,
    left: (width - 300) / 2,
    top: (height - 350) / 2,
  },
  placeholderKeyboardVisible: {
    top: (height - 550) / 2,
  },
  placeholderText: {
    width: 300,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
    marginTop: 10,
  },
  handle: {
    fontWeight: 'bold',
  },
  bubbleWrapper: {
    position: 'relative',
    width: '100%',
  },
  bubbleSpacing: (firstUnread, moderatorStart) => {
    let paddingTop = 0;
    if (firstUnread && moderatorStart) {
      paddingTop = 100;
    } else if (firstUnread) {
      paddingTop = 52;
    } else if (moderatorStart) {
      paddingTop = 62;
    }
    return { paddingTop };
  },
  avatarWrapper: {
    padding: Platform.OS === 'android' ? 3 : 0,
  },
  unreadSeparator: (moderatorJoined = false, position) => ({
    position: 'absolute',
    width: width - 32,
    top: moderatorJoined ? 40 : 0,
    left: position === 'left' ? -36 : 16,
    marginRight: 16,
    borderTopWidth: 1,
    borderTopColor: brandColor,
    paddingTop: 4,
    marginBottom: 12,
    marginTop: 16,
    alignItems: 'center',
  }),
  unreadText: {
    color: brandColor,
    fontSize: 12,
  },
  moderatorNotificationWrapper: position => ({
    position: 'absolute',
    width: width - 32,
    top: 0,
    left: position === 'left' ? -36 : 16,
    marginBottom: 12,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
  }),
  fixedModeratorNotificationWrapper: {
    width: width - 32,
    marginHorizontal: 16,
    marginVertical: 12,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
  },
  moderatorNotification: {
    paddingVertical: 8,
    paddingHorizontal: 16,
    fontSize: 14,
    color: primaryTextColor,
    textAlign: 'center',
    backgroundColor: secondaryBackgroundColor,
    borderRadius: 5,
  },
  bold: {
    fontWeight: 'bold',
  },
  listViewStyle: {
    backgroundColor: '#FFFFFF',
  },
  footerContainer: {
    marginTop: 5,
    marginHorizontal: 16,
    marginBottom: 10,
    flexDirection: 'row',
    alignItems: 'center',
    height: 32,
  },
  footerText: {
    marginLeft: 10,
    fontSize: 14,
    color: '#aaa',
  },
  sendStyle: {
    color: 'rgb(0, 191, 101)',
  },
  avatar: {
    width: Platform.OS === 'android' ? 30 : 36,
  },
  chatLeftBubbleStyle: {
    backgroundColor: foregroundColor,
    borderWidth: 1,
    borderColor: mainBorderColor,
  },
  rightTextStyle: {
    color: 'black',
  },
  emptyStateAvatar: {
    width: 50,
  },
  footerAvatar: {
    width: 25,
  },
  blockedMessageInput: {
    height: 64,
    width: '100%',
    backgroundColor: 'white',
    justifyContent: 'center',
    paddingHorizontal: 16,
    borderTopWidth: 1,
    borderTopColor: borderColor,
  },
  blockedMessage: {
    color: warningColor,
  },
};

// firstUnread: This is position for showing Unread position in the UI
// showTypingTimerId: This is the timerId which sends the request to backend for typing request
// recievedMessages: This has the list of messages coming through websocket
// messages: List of messages to show in the UI

class ChatDetail extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      messages: [],
      recievedMessages: [],
      keyboardVisible: false,
      firstUnread: -1,
      timerId: -1,
      typing: false,
      showTypingTimerId: -1,
      isMounted: false,
      inDispute: props.inDispute,
      text: props.listingUrl || '',
    };
  }

  componentDidMount() {
    const {
      fetchChatDetail,
      setChatDetailLoading,
      fetchChats,
      profile,
      isFromOrder,
      peerID,
      subject,
      moderatorId,
      buyerId,
      vendorId,
      isBlocked,
    } = this.props;
    const { inDispute } = this.state;

    this.setState({ isMounted: true, recievedMessages: [] });
    // Fetch current chat history
    fetchChatDetail({ peerID, subject, isFromOrder });
    if (!this.isSameChat()) {
      // This section is for showing loading indicator.
      // When user navigate back to chat list and again to the same chat detail, then no need to
      // show loading indicator
      setChatDetailLoading(true);
    }

    this.setInitialMessage([]);

    // const { username, password } = this.props.appstate;
    // const socketUrl = `ws://${username}:${password}@${websocketHost}/ws`;
    const socketUrl = `ws://${websocketHost}/ws`;
    this.ws = new global.WebSocket(socketUrl, '', { headers: serverConfig.getAuthHeader() });

    this.ws.onmessage = (e) => {
      if (isBlocked) {
        return;
      }

      const data = JSON.parse(e.data);
      const { message, messageTyping } = data;
      if (!message) {
        if ((messageTyping && messageTyping.peerId === peerID) ||
          (
            inDispute &&
            messageTyping &&
            [moderatorId, buyerId, vendorId].includes(messageTyping.peerId))
        ) {
          // This is for showing typing indicator if the messageTyping response comes through
          // websocket, then it sets the type state to true for 2 sec
          if (this.state.showTypingTimerId) {
            clearTimeout(this.state.showTypingTimerId);
          }
          const showTypingTimerId = setTimeout(() => {
            this.setState({ typing: false, showTypingTimerId: -1 });
          }, 2000);
          this.setState({ typing: true, showTypingTimerId });
        } else {
          this.setState({ typing: false });
        }
        return;
      }
      // When message response is recieved, then updates the chats to update the chat badges
      if (message.subject === subject && message.peerId === moderatorId) {
        fetchChats();
        fetchChatDetail({ peerID, subject, isFromOrder });
        if (!inDispute) {
          this.setState({ inDispute: true });
          this.appendMessage(profile, { ...message, moderatorJoined: true });
        } else {
          this.appendMessage(profile, { ...message });
        }
      }
      if (message.subject === subject && [buyerId, vendorId, peerID].includes(message.peerId)) {
        // When message is for current chat then it will be added to current message list.
        this.appendMessage(profile, message);
      }
    };

    if (Platform.OS === 'ios') {
      this.keyboardDidShowSub = Keyboard.addListener('keyboardWillShow', this.keyboardDidShow);
      this.keyboardWillHideSub = Keyboard.addListener('keyboardWillHide', this.keyboardWillHide);
    } else {
      this.keyboardDidShowSub = Keyboard.addListener('keyboardDidShow', this.keyboardDidShow);
      this.keyboardWillHideSub = Keyboard.addListener('keyboardDidHide', this.keyboardWillHide);
    }
  }

  componentDidUpdate(prevProps) {
    const {
      profile,
      loading,
      chatDetail,
      setChatAsRead,
      isFromOrder,
      peerID,
      subject,
      isBlocked,
      fetchChatDetail,
      listingUrl,
    } = this.props;
    const { recievedMessages } = this.state;

    let mergingMessages = [];
    // receviedMessages is the list of messages through websocket
    if (peerID !== prevProps.peerID || subject !== prevProps.subject) {
      // If it's not same chat then need to reset mergineMessages
      mergingMessages = [];
    } else {
      // If it's the same chat then need to set mergingMessages as recievedMessages
      mergingMessages = recievedMessages;
    }

    if (prevProps.loading && !loading) {
      // set message list and unread position
      const firstUnread = this.getFirstUnreadPos(chatDetail);
      const initialMessages =
        chatDetail.map(convertMsgToGiftChatItem(profile, this.getPeerProfile));
      this.setState({ messages: initialMessages, firstUnread });
      setChatAsRead({ peerID, subject, isFromOrder });
    }

    if (!deepEqual(prevProps.chatDetail, chatDetail)) {
      // set initial message list
      this.setInitialMessage(mergingMessages);
    }

    // if node is unblocked, fetch chat detail again
    if (prevProps.isBlocked && !isBlocked) {
      fetchChatDetail({ peerID, subject, isFromOrder });
    }

    if (prevProps.listingUrl !== listingUrl) {
      this.setState({ text: listingUrl || '' });
    }
  }

  componentWillUnmount() {
    this.setState({ isMounted: false });
    if (this.state.timerId !== -1) {
      clearInterval(this.state.timerId);
    }
    if (this.state.showTypingTimerId !== -1) {
      clearInterval(this.state.showTypingTimerId);
    }
    this.keyboardDidShowSub.remove();
    this.keyboardWillHideSub.remove();
  }

  setInitialMessage = (mergingMessages) => {
    // This function sets the unread position
    // Most importantly it sets the inital list for messages
    // When the messages are coming through websockets, then those messages will be added to list
    const {
      profile, chatDetail, disputeTimestamp,
    } = this.props;
    const { inDispute } = this.state;
    const initialMessages = chatDetail.map(convertMsgToGiftChatItem(profile, this.getPeerProfile));
    const firstUnread = this.getFirstUnreadPos(chatDetail);
    const result = [...initialMessages, ...mergingMessages];
    if (inDispute) {
      const lastIndex =
        result
          .map(msg => moment(disputeTimestamp).isBefore(moment(msg.createdAt)))
          .lastIndexOf(true);
      result[lastIndex] = { ...result[lastIndex], moderatorJoined: true };
    }
    _.sortBy(result, o => moment(o.createdAt).valueOf());
    this.setState({
      messages: result,
      recievedMessages: mergingMessages,
      firstUnread,
    });
  }

  getPeerProfile = (userPeerID) => {
    const { profiles, peerID } = this.props;
    const profileID = userPeerID || peerID;
    return (profiles && profiles[profileID]) || getDefaultProfileFromPeerId(profileID);
  }

  getFirstUnreadPos = chatDetail => _.reduceRight(
    chatDetail,
    (result, item) => {
      if (!item.outgoing && !item.read && result === -1) {
        return item.messageId;
      }
      return result;
    },
    -1,
  )

  handleInputChange = (text) => {
    const { listingUrl } = this.props;
    const { text: originText } = this.state;
    if (originText !== listingUrl || text !== '') {
      this.setState({ text });
    }
  }

  handleSend = (messages = []) => {
    const {
      peerID: targetPeerID,
      subject,
      sendChat,
      isFromOrder,
      moderatorId,
      vendorId,
      buyerId,
      profile: { peerID: profilePeerID },
      orderType,
    } = this.props;
    const { inDispute, keyboardVisible } = this.state;
    const message = messages[0];
    const localId = message === '' ? profilePeerID : message.user._id;

    let peerID;
    if (isFromOrder) {
      switch (localId) {
        case moderatorId:
          peerID = [vendorId, buyerId];
          break;
        case vendorId:
          if (moderatorId && inDispute) {
            peerID = [moderatorId, buyerId];
          } else {
            peerID = buyerId;
          }
          break;
        case buyerId:
        default:
          if (moderatorId && inDispute) {
            peerID = [moderatorId, vendorId];
          } else {
            peerID = vendorId;
          }
          break;
      }
    } else {
      peerID = targetPeerID;
    }
    if (message === '') {
      if (keyboardVisible) {
        sendChat({ peerID, message, localId, subject, orderType });
      }
    } else {
      sendChat({
        peerID,
        message: message.text,
        localId,
        subject,
        onSent: this.messageSent(localId),
        orderType,
      });

      const msg = { ...message, loading: true, moderatorJoined: false };

      this.setState((prevState) => {
        const newMessages = GiftedChat.append(prevState.messages, msg, true);
        return { messages: newMessages, firstUnread: -1 };
      });
    }
  }

  handleScrollToEnd = () => {
    const {
      setChatAsRead, isFromOrder, peerID, subject,
    } = this.props;
    this.setState({ firstUnread: -1 });
    setChatAsRead({ peerID, subject, isFromOrder });
  };

  handlePressAvatar = user => () => {
    this.props.navigation.navigate('ExternalStore', { peerID: user._id });
  };

  appendMessage = (profile, message) => {
    // This function is for append incoming message through websocket to current message list.
    this.setState((prevState) => {
      const pos = prevState.messages.findIndex(msg => msg.messageId === message.messageId);
      let messages = [];
      let recievedMessages = [];
      if (pos >= 0) {
        // When incomming message exists in the chat history (in case of offline messages this
        // happens) then only update the timestamp field for that message in existing history
        // because message from websocket has more accurate timestamp data
        messages = prevState.messages.map((msg) => {
          if (message.messageId === msg.messageId) {
            return {
              ...msg,
              timestamp: message.timestamp,
            };
          }
          return { ...msg };
        });
        recievedMessages = [
          ...prevState.recievedMessages,
          convertMsgToGiftChatItem(profile, this.getPeerProfile)(message),
        ];
      } else {
        // If not exists, then just append it to the back of the message list
        messages = GiftedChat.append(
          prevState.messages,
          convertMsgToGiftChatItem(profile, this.getPeerProfile)(message),
          true,
        );
        recievedMessages = [
          ...prevState.recievedMessages,
          convertMsgToGiftChatItem(profile, this.getPeerProfile)(message),
        ];
      }
      // Make sure that the message list is in order, becase in case of offline messages
      // the messages are not comming in order
      return {
        messages,
        recievedMessages: _.orderBy(recievedMessages, [o => moment(o.createdAt).valueOf()]),
        type: false,
      };
    });
  }

  isSameChat = () => {
    const { peerID, subject, isFromOrder, currentChatInfo } = this.props;
    const {
      peerID: currentPeerID,
      subject: currentSubject,
      isFromOrder: currentIsFromOrder,
    } = currentChatInfo;
    return (
      currentPeerID === peerID &&
      currentSubject === subject &&
      currentIsFromOrder === isFromOrder
    );
  }

  keyboardDidShow = () => {
    if (this.state.timerId === -1) {
      const timerId = setInterval(() => this.handleSend(['']), 1000);
      this.setState({ keyboardVisible: true, timerId });
    }
  };

  keyboardWillHide = () => {
    const { timerId } = this.state;
    clearInterval(timerId);
    this.setState({ keyboardVisible: false, timerId: -1 });
  };

  goBack = () => {
    const {
      navigation, setChatAsRead, isFromOrder, peerID, subject,
    } = this.props;
    setChatAsRead({ peerID, subject, isFromOrder });
    navigation.goBack();
  };

  messageSent = msgId => (success) => {
    const { isMounted } = this.state;
    if (isMounted) {
      const messages = this.state.messages.map(message => ({ ...message }));
      const message = messages.find(message => message._id === msgId);
      if (message) {
        message.loading = false;
        message.success = success;
      }
      this.setState({ messages });
    }
  }

  handleMessagePress = (msgId, loading, success) => () => {
    if (loading === false && success === false) {
      const {
        sendChat, isFromOrder, peerID, subject, orderType,
      } = this.props;
      const message = this.state.messages.find(msg => msg._id === msgId);
      const messageText = message.text;
      const messageInfo = { ...message, loading: true, createdAt: moment().format() };
      const messages = this.state.messages.filter(msg => msg._id !== msgId);
      this.setState({ messages: GiftedChat.append(messages, messageInfo, true) });
      sendChat({
        peerID,
        message: messageText,
        localId: msgId,
        subject,
        onSent: this.messageSent(msgId),
        isFromOrder,
        orderType,
      });
    }
  }

  handleModeratorTap = () => {
    const { moderatorId, navigation } = this.props;
    navigation.push('ModeratorDetails', { moderator: moderatorId });
  }

  decodeTextInMessages = message => ({ ...message, text: decode(message.text) })

  renderAvatar = (props) => {
    const { currentMessage, onPressAvatar } = props;
    const { user, _id: currentMessageId, moderatorJoined } = currentMessage;
    const { firstUnread } = this.state;
    return (
      <TouchableWithoutFeedback onPress={onPressAvatar(user)}>
        <View
          style={[
            styles.avatarWrapper,
            styles.bubbleSpacing(currentMessageId === firstUnread, moderatorJoined),
          ]}
        >
          <AvatarImage thumbnail={user.avatar} style={styles.avatar} />
        </View>
      </TouchableWithoutFeedback>
    );
  };

  renderFooter = () => {
    const { typing } = this.state;
    const peerProfile = this.getPeerProfile();
    const avatar = _.get(peerProfile, 'avatarHashes.tiny');
    return (
      <View
        style={[
          styles.footerContainer,
          {
            height: typing ? 32 : 0,
            opacity: typing ? 1 : 0,
          },
        ]}
      >
        <AvatarImage thumbnail={avatar} style={styles.footerAvatar} />
        <Text style={styles.footerText}>{I18n.t('components.templates.ChatDetail.is_typing', {peer: peerProfile.name})}</Text>
      </View>
    );
  };

  renderLoading = () => <ActivityIndicator size="large" />;

  renderMessageText = props => (
    <MessageText {...props} />
  )

  renderBubble = (props) => {
    const { moderatorId, vendorId, buyerId, isFromOrder } = this.props;
    const { firstUnread, inDispute } = this.state;
    const { position, currentMessage } = props;
    const {
      _id: currentMessageId, moderatorJoined, loading = false, success = true,
    } = currentMessage;

    return (
      <View
        style={[
          styles.bubbleWrapper,
          styles.bubbleSpacing(currentMessageId === firstUnread, moderatorJoined),
        ]}
      >
        {inDispute && moderatorJoined && this.renderModeratorJoinedIndicator(false, position)}
        {currentMessageId === firstUnread && (
          <View style={styles.unreadSeparator(moderatorJoined, position)}>
            <Text style={styles.unreadText}>{I18n.t('components.templates.ChatDetail.unread')}</Text>
          </View>
        )}
        <ChatBubble
          {...props}
          loading={loading}
          success={success}
          onPress={this.handleMessagePress(currentMessageId, loading, success)}
          renderMessageText={this.renderMessageText}
          wrapperStyle={{ left: styles.chatLeftBubbleStyle }}
          textStyle={{ right: styles.rightTextStyle }}
          orderProps={{ moderatorId, vendorId, buyerId, isFromOrder }}
        />
      </View>
    );
  };

  renderSend = props => (
    <Send textStyle={styles.sendStyle} containerStyle={styles.sendContainerStyle} {...props} />
  );

  renderInputToolbar = (props) => {
    if (this.props.isBlocked) {
      return (
        <View style={styles.blockedMessageInput}>
          <Text style={styles.blockedMessage}>{I18n.t('components.templates.ChatDetail.block_message')}</Text>
        </View>
      );
    } else {
      return <InputToolbar {...props} />;
    }
  }

  renderEmptyState = () => {
    const { inDispute } = this.state;
    if (inDispute) {
      return false;
    }
    const { keyboardVisible } = this.state;
    const peerProfile = this.getPeerProfile();
    const thumbnail = _.get(peerProfile, 'avatarHashes.small');
    const { handle = '', name = 'Anonymous' } = peerProfile;
    return (
      <View
        style={[styles.placeholderWrapper, keyboardVisible && styles.placeholderKeyboardVisible]}
      >
        <AvatarImage thumbnail={thumbnail} style={styles.emptyStateAvatar} />
        <Text style={styles.placeholderText}>
        {I18n.t('components.templates.ChatDetail.start_with') + ' '}
          <Text style={styles.handle}>{handle ? `@${handle}` : decode(name)}</Text>
        </Text>
      </View>
    );
  }

  renderModeratorJoinedIndicator = (fixed = false, position = 'left') => {
    const { moderatorId, isModerated, isFromOrder } = this.props;
    const { name = 'Anonymous' } = this.getPeerProfile(moderatorId);
    return isModerated && isFromOrder && (
      <TouchableWithoutFeedback onPress={this.handleModeratorTap}>
        <View
          style={fixed ? (
            styles.fixedModeratorNotificationWrapper
          ) : (
            styles.moderatorNotificationWrapper(position)
          )}
        >
          <Text style={styles.moderatorNotification}>
            <Text style={styles.bold}>{name}</Text> {I18n.t('components.templates.ChatDetail.moderator_join')}
          </Text>
        </View>
      </TouchableWithoutFeedback>
    );
  }

  renderMainContent() {
    const { profile, loading } = this.props;
    const { messages, inDispute, text } = this.state;
    const { paddingBottom, ...wrapperStyle } = wrapperStyles.bottomWrapper;
    return (
      <View style={[wrapperStyle, { foregroundColor }]}>
        {loading && this.renderLoading()}
        {!loading && inDispute && _.isEmpty(messages) && this.renderModeratorJoinedIndicator(true)}
        {!loading && (
          <GiftedChat
            text={text}
            messages={messages.map(this.decodeTextInMessages)}
            onSend={this.handleSend}
            onInputTextChanged={this.handleInputChange}
            user={{ _id: profile.peerID }}
            listViewProps={{
              style: styles.listViewStyle,
              onEndReached: this.handleScrollToEnd,
            }}
            onPressAvatar={this.handlePressAvatar}
            renderAvatarOnTop
            renderAvatar={this.renderAvatar}
            renderBubble={this.renderBubble}
            renderSend={this.renderSend}
            renderInputToolbar={this.renderInputToolbar}
            renderChatFooter={this.renderFooter}
            placeholder={I18n.t('components.templates.ChatDetail.say_something_nice')}
            minInputToolbarHeight={60}
            maxComposerHeight={120}
            textInputProps={{ autoFocus: true }}
          />
        )}
        {!loading && <View style={{ height: paddingBottom, backgroundColor: 'white' }} />}
        {!loading && _.isEmpty(messages) && this.renderEmptyState()}
      </View>
    );
  }

  render() {
    const { loading } = this.props;
    if (!loading && _.isEmpty(this.state.messages)) {
      return (
        <TouchableWithoutFeedback onPress={Keyboard.dismiss}>
          {this.renderMainContent()}
        </TouchableWithoutFeedback>
      );
    } else {
      return this.renderMainContent();
    }
  }
}

const mapStateToProps = state => ({
  chatDetail: state.chat.chatDetail,
  currentChatInfo: state.chat.currentChatInfo,
  loading: state.chat.detailLoading,
  profiles: state.profiles,
  profile: state.profile.data,
  appstate: state.appstate,
});

const mapDispatchToProps = {
  fetchChatDetail,
  sendChat,
  blockNode,
  setChatAsRead,
  fetchChats,
  setChatDetailLoading,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
)(ChatDetail));
