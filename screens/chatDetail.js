import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert } from 'react-native';
import Toast from 'react-native-root-toast';
import decode from 'unescape';
import * as _ from 'lodash';

import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import ChatDetailTemplate from '../components/templates/ChatDetail';
import StoreMoreMenu from '../components/molecules/StoreMoreMenu';
import { foregroundColor } from '../components/commonColors';
import OBActionSheet from '../components/organism/ActionSheet';

import {
  fetchChats,
  sendChat,
  setChatAsRead,
  setChatDetailLoading,
  deleteChatConversation,
} from '../reducers/chat';
import { blockNode, unblockNode } from '../reducers/settings';
import { followPeer, unfollowPeer } from '../reducers/follow';

import { wrapperStyles } from '../utils/styles';
import { getDefaultProfileFromPeerId } from '../utils/profile';
import { TOAST_OPTION } from '../utils/toast';

import {I18n} from '../langs/I18n';
class ChatDetail extends PureComponent {
  state = {
  };

  getPeerProfile = () => {
    const { profiles, navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    return (profiles && profiles[peerID]) || getDefaultProfileFromPeerId(peerID);
  }

  setActionSheet = (component) => {
    this.actionSheet = component;
  };

  handleShowActionSheet = () => {
    this.actionSheet.show();
  };

  isFollowing = () => {
    const { followingsFromLocal, navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    return followingsFromLocal.includes(peerID);
  }

  isBlocked = () => {
    const { blockedNodes, navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    return blockedNodes.includes(peerID);
  };

  handleActionSheet = (index) => {
    const { navigation, blockNode, unblockNode, followPeer, unfollowPeer } = this.props;
    const peerID = navigation.getParam('peerID');
    const isBlocked = this.isBlocked();
    const isFollowing = this.isFollowing();
    switch (index) {
      case 0:
        navigation.navigate('ExternalStore', { peerID });
        break;
      case 1:
        if (isBlocked) {
          unblockNode(peerID);
        } else {
          blockNode(peerID);
        }
        break;
      case 2:
        if (isFollowing) {
          unfollowPeer(peerID);
          Toast.show(I18n.t('screens.chatDetail.Unfollowed'), TOAST_OPTION);
        } else {
          followPeer(peerID);
          Toast.show(I18n.t('screens.chatDetail.Followed'), TOAST_OPTION);
        }
        break;
      case 3:
        Alert.alert(I18n.t('screens.chatDetail.Delete_conversation'), I18n.t('screens.chatDetail.cannot_undo'), [
          { text: I18n.t('screens.chatDetail.Cancel') },
          {
            text: I18n.t('screens.chatDetail.Delete'),
            onPress: this.deleteChatConversation,
          },
        ]);
        break;
      default:
        break;
    }
  };

  deleteChatConversation = () => {
    const { navigation, deleteChatConversation } = this.props;
    const peerID = navigation.getParam('peerID');
    deleteChatConversation(peerID);
    navigation.pop();
  }

  goBack = () => {
    const peerID = this.props.navigation.getParam('peerID');
    const subject = this.props.navigation.getParam('subject');
    this.props.setChatAsRead({ peerID, subject });
    this.props.navigation.goBack();
  };

  render() {
    const { paddingBottom, ...wrapperStyle } = wrapperStyles.bottomWrapper;
    const peerID = this.props.navigation.getParam('peerID');
    const subject = this.props.navigation.getParam('subject');
    const listingUrl = this.props.navigation.getParam('listingUrl');
    const peerProfile = this.getPeerProfile();
    const isBlocked = this.isBlocked();
    const isFollowing = this.isFollowing();
    return (
      <View style={[wrapperStyle, { foregroundColor }]}>
        <Header
          left={<NavBackButton />}
          onLeft={this.goBack}
          title={decode(_.get(peerProfile, 'name')) || 'Chat'}
          right={<StoreMoreMenu onMore={this.handleShowActionSheet} black />}
        />
        <ChatDetailTemplate peerID={peerID} subject={subject} listingUrl={listingUrl} isBlocked={isBlocked} />
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleActionSheet}
          options={[
            I18n.t('screens.chatDetail.Go_to_profile'),
            isBlocked ? I18n.t('screens.chatDetail.Unblock_user') : I18n.t('screens.chatDetail.Block_user'),
            isFollowing ? I18n.t('screens.chatDetail.Unfollow') : I18n.t('screens.chatDetail.Follow'),
            I18n.t('screens.chatDetail.Delete_conversation'),
            I18n.t('screens.chatDetail.Cancel'),
          ]}
          destructiveButtonIndex={3}
          cancelButtonIndex={4}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  chatDetail: state.chat.chatDetail,
  loading: state.chat.detailLoading,
  profiles: state.profiles,
  profile: state.profile.data,
  appstate: state.appstate,
  blockedNodes: state.settings.blockedNodes,
  followingsFromLocal: state.follow.followingsFromLocal,
});

const mapDispatchToProps = {
  sendChat,
  blockNode,
  unblockNode,
  setChatAsRead,
  fetchChats,
  setChatDetailLoading,
  deleteChatConversation,
  followPeer,
  unfollowPeer,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ChatDetail);
