import React from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback, Share, Alert } from 'react-native';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import Feather from 'react-native-vector-icons/Feather';
import { withNavigation } from 'react-navigation';
import decode from 'unescape';

import {
  getListingIfExists,
  getListingLinkFromContent,
  fetchListingLink,
  handlePressListing,
  renderContentWithListingPreview,
  handleDeeplinkPress,
  handleCrypoLink,
  handleHashtag,
  getFeedInfo,
  getStatusToDecode,
} from '../organism/ContentWithListingPreview';
import AvatarImage from '../atoms/AvatarImage';
import FeedItemFooter from './FeedItemFooter';
import FeedImageGrid from './FeedImageGrid';
import FeedPreview from './FeedPreview';
import OBActionSheet from '../organism/ActionSheet';
import { foregroundColor, primaryTextColor, secondaryTextColor, brandColor } from '../commonColors';
import { timeSince } from '../../utils/time';

import { deleteFeedItem, removeFeedItem, fetchGlobalFeedItem } from '../../reducers/feed';
import { showToast, hideToast } from '../../reducers/appstate';
import { fetchListing } from '../../reducers/listings';
import { blockNode } from '../../reducers/settings';
import { cellStyles, chatStyles } from '../../utils/styles';
import { createFeedUrlFromPeerIDAndSlug } from '../../utils/navigation';

import { eventTracker } from '../../utils/EventTracker';

import { getActivity } from '../../selectors/stream';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'transparent',
    flexDirection: 'column',
  },
  contentContainer: {
    paddingTop: 6,
    backgroundColor: foregroundColor,
    flexDirection: 'row',
    borderBottom: 1,
    borderColor: 'rgb(200, 199, 204)',
  },
  nonAvatarContainer: {
    flexDirection: 'column',
    flex: 1,
  },
  headerContainer: {
    height: 20,
    marginTop: 8,
    marginBottom: 3,
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  header: {
    paddingHorizontal: 12,
    flex: 1,
    flexDirection: 'row',
    alignItems: 'flex-end',
  },
  text: {
    fontSize: 17,
    lineHeight: 24,
    color: '#000000',
    paddingHorizontal: chatStyles.avatarImage.marginLeft,
  },
  moreIcon: {
    width: 32,
    height: 32,
    alignItems: 'center',
  },
  footer: {
    paddingLeft: chatStyles.avatarImage.marginLeft,
  },
  timestamp: {
    color: secondaryTextColor,
    fontSize: 13,
    paddingBottom: 1,
  },
  spacer: {
    height: 10,
    backgroundColor: 'white',
  },
  previewWrapper: {
    paddingTop: 10,
    paddingHorizontal: 16,
  },
  buttonIcon: {
    marginRight: 7,
  },
  repostIndicator: {
    color: 'white',
    flexDirection: 'row',
    alignItems: 'center',
    height: 30,
    paddingHorizontal: 15,
    paddingTop: 10,
  },
  providerName: {
    fontSize: 14,
    fontWeight: 'bold',
    color: brandColor,
    marginRight: 5,
  },
  repostText: {
    fontSize: 14,
    color: primaryTextColor,
  },
  linkText: {
    color: brandColor,
  },
};

class FeedItem extends React.PureComponent {
  state = {}

  constructor(props) {
    super(props);

    this.getListingIfExists = getListingIfExists.bind(this);
    this.getListingLinkFromContent = getListingLinkFromContent.bind(this);
    this.fetchListingLink = fetchListingLink.bind(this);
    this.handlePressListing = handlePressListing.bind(this);
    this.renderContentWithListingPreview = renderContentWithListingPreview.bind(this);
    this.handleDeeplinkPress = handleDeeplinkPress.bind(this);
    this.handleCrypoLink = handleCrypoLink.bind(this);
    this.handleHashtag = handleHashtag.bind(this);
    this.getFeedInfo = getFeedInfo.bind(this);
    this.getStatusToDecode = getStatusToDecode.bind(this);
    this.styles = styles;
  }

  componentDidMount() {
    if (this.props.activityId) {
      this.fetchListingLink();
    }
  }

  componentDidUpdate(prevProps) {
    if (this.getStatusToDecode(prevProps) !== this.getStatusToDecode(this.props)) {
      this.fetchListingLink();
    }
  }

  getRepostFeedInfo = (propsFromParam) => {
    const props = propsFromParam || this.props;
    const { getActivity } = props;
    const feedInfo = this.getFeedInfo(propsFromParam);
    const activityId = _.get(feedInfo, 'postData.activityId');
    return getActivity(activityId);
  }

  getRepostedFeed = () => {
    const item = this.getFeedInfo();
    return _.get(item, 'postData.originPost.data', {});
  }

  getReposterProfile = () => {
    const { reposter = {} } = this.getFeedInfo() || {};
    return reposter.data || {};
  }

  getUserProfile = () => {
    const { actor } = this.getFeedInfo() || {};
    return actor.data || {};
  }

  setActionSheet = (component) => {
    this.actionSheet = component;
  };

  isRepost = (propsFromParam) => {
    const props = propsFromParam || this.props;
    return this.getFeedInfo(props).verb === 'REPOST';
  }

  handleShowActionSheet = () => {
    this.actionSheet.show();
  }

  handlePressAvatar = () => {
    const { navigation, profile: { peerID: currentPeerID } } = this.props;
    const profile = this.getUserProfile();
    eventTracker.trackEvent('Social-TappedAvatar');
    if (profile.peerID === currentPeerID) {
      navigation.navigate('MyStore');
    } else {
      navigation.push('ExternalStore', { peerID: profile.peerID });
    }
  }

  handlePressOriginalPostOwner = () => {
    const { profile: { peerID: currentPeerID }, navigation } = this.props;
    const item = this.getFeedInfo();

    const { status } = _.get(item, 'object.data.post', {});
    if (status === '') {
      let posterProfile = this.getReposterProfile();
      if (!posterProfile.peerID) {
        posterProfile = this.getUserProfile();
      }
      if (posterProfile.peerID === currentPeerID) {
        navigation.navigate('MyStore');
      } else {
        navigation.push('ExternalStore', { peerID: posterProfile.peerID });
      }
    } else {
      this.handlePressAvatar();
    }
  }

  handleDelete = () => {
    const { deleteFeedItem, navigation } = this.props;
    const item = this.getFeedInfo();
    const { actor, id: activityId, verb } = item || {};
    const { id: peerID } = actor;
    const slug = _.get(item, 'object.data.post.slug');
    if (verb === 'POST') {
      deleteFeedItem({ peerID, slug, activityId });
    } else {
      const reactionId = _.get(item, 'postData.reactionId');
      deleteFeedItem({ peerID, slug, activityId, reactionId });
    }
    showToast({ type: 'message', message: 'Post deleted' });
    setTimeout(hideToast, 3000);
    if (navigation.state.routeName === 'FeedDetail') {
      const screenKey = navigation.getParam('screenKey');
      if (screenKey) {
        navigation.pop(2);
      } else {
        navigation.goBack();
      }
    }
  };

  handleRemove = () => {
    const { removeFeedItem, navigation, showToast, hideToast } = this.props;
    const item = this.getFeedInfo();
    const { actor, id: activityId, verb } = item || {};
    const { id: peerID } = actor;
    const slug = _.get(item, 'object.data.post.slug');
    if (verb === 'POST') {
      removeFeedItem({ peerID, slug, activityId });
    } else {
      const reactionId = _.get(item, 'postData.reactionId');
      removeFeedItem({ peerID, slug, activityId, reactionId });
    }
    showToast({ type: 'message', message: 'Post hidden' });
    setTimeout(hideToast, 3000);
    if (navigation.state.routeName === 'FeedDetail') {
      const screenKey = navigation.getParam('screenKey');
      if (screenKey) {
        navigation.pop(2);
      } else {
        navigation.goBack();
      }
    }
  };

  handleChange = (index) => {
    const { navigation, reportUser, blockNode } = this.props;
    const item = this.getFeedInfo();
    const { actor, id } = item || {};
    const { id: peerID } = actor;
    const { status } = _.get(item, 'object.data', {});
    if (this.isFeedOwner()) {
      switch (index) {
        case 0:
          eventTracker.trackEvent('Social-SharedPost');
          Share.share({
            message: createFeedUrlFromPeerIDAndSlug(peerID, id),
            title: status,
          });
          break;
        case 1:
          Alert.alert(I18n.t('components.molecules.FeedItem.delete_post'), I18n.t('components.molecules.FeedItem.undo_action1'), [
            { text: I18n.t('components.molecules.FeedItem.cancel1')},
            {
              text: I18n.t('components.molecules.FeedItem.delete1'),
              onPress: this.handleDelete,
            },
          ]);
          break;
        default:
          break;
      }
    } else {
      switch (index) {
        case 0:
          eventTracker.trackEvent('Social-SharedPost');
          Share.share({
            message: createFeedUrlFromPeerIDAndSlug(peerID, id),
            title: status,
          });
          break;
        case 1:
          navigation.navigate('ExternalStore', { peerID });
          break;
        case 2:
          if (reportUser) {
            reportUser(peerID);
          }
          break;
        case 3:
          blockNode(peerID);
          break;
        case 4:
          Alert.alert(I18n.t('components.molecules.FeedItem.hide_post'), I18n.t('components.molecules.FeedItem.undo_action2'), [
            { text: I18n.t('components.molecules.FeedItem.cancel2')},
            {
              text: I18n.t('components.molecules.FeedItem.hide'),
              onPress: this.handleRemove,
            },
          ]);
          break;
        default:
          break;
      }
    }
  };

  handlePostNavigation = () => {
    const { onPress } = this.props;
    if (this.isRepost()) {
      const item = this.getFeedInfo();
      const { status } = _.get(item, 'object.data.post', {});
      const repostActivityId = item.id;
      const originActivityId = _.get(item, 'postData.activityId');

      const activityId = status === '' ? originActivityId : repostActivityId;
      onPress(activityId, 'comment', false);
    } else {
      const { activityId } = this.props;
      onPress(activityId, 'comment', false);
    }
  }

  isFeedOwner = () => {
    const { profile: { peerID } } = this.props;
    const { actor } = this.getFeedInfo();
    return actor.id === peerID || actor === peerID;
  }

  renderHeader = (name, timestamp) => {
    const { hideMoreButton } = this.props;
    return (
      <View style={styles.headerContainer}>
        <View style={styles.header}>
          <TouchableWithoutFeedback onPress={this.handlePressAvatar}>
            <Text style={chatStyles.name} numberOfLines={1}>
              {`${decode(name)}`}
            </Text>
          </TouchableWithoutFeedback>
          <Text style={styles.timestamp}>
            {timeSince(new Date(timestamp))}
          </Text>
        </View>
        {!hideMoreButton && (
          <TouchableWithoutFeedback onPress={this.handleShowActionSheet}>
            <View style={styles.moreIcon}>
              <Ionicons name="md-more" size={24} color={primaryTextColor} />
            </View>
          </TouchableWithoutFeedback>
        )}
      </View>
    );
  }

  renderBottom = () => {
    const { noSeparator } = this.props;
    const isFeedOwner = this.isFeedOwner();
    return (
      <React.Fragment>
        <View style={styles.spacer} />
        {!noSeparator && <View style={cellStyles.separator} />}
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={
            isFeedOwner ? (
              [I18n.t('components.molecules.FeedItem.share_to1'), I18n.t('components.molecules.FeedItem.delete2'), I18n.t('components.molecules.FeedItem.cancel3')]
            ) : (
                [I18n.t('components.molecules.FeedItem.share_to2'), I18n.t('components.molecules.FeedItem.go_to_profile'), I18n.t('components.molecules.FeedItem.report_user'), I18n.t('components.molecules.FeedItem.block_user'), I18n.t('components.molecules.FeedItem.hide_post'), I18n.t('components.molecules.FeedItem.cancel4')]
              )}
          destructiveButtonIndex={isFeedOwner ? 1 : 2}
          cancelButtonIndex={isFeedOwner ? 2 : 5}
        />
      </React.Fragment>
    );
  }

  render() {
    if (_.get(this.getFeedInfo(), 'object.error') === 'ReferenceNotFound') {
      return null;
    }

    const {
      style, onComment, onViewImages, repost, deleteRepost,
    } = this.props;

    const item = this.getFeedInfo();

    if (_.isEmpty(item)) {
      return null;
    }

    // Post variables
    const profile = this.getUserProfile();
    const { timestamp, images = [], status } = _.get(item, 'object.data.post', {});
    const shouldRenderRepost = this.isRepost() && status === '';

    // Repost variables
    const { time } = item;
    const { activityId } = item.postData;
    const originActivity = this.getRepostFeedInfo();
    const originPost = _.get(originActivity, 'object.data.post', {});
    const reposterProfile = this.getReposterProfile();
    const { name: profileName = 'Anonymous' } = profile || {};
    const { name: reposterName = 'Anonymous' } = reposterProfile || {};
    const name = shouldRenderRepost ? reposterName : profileName;
    const { images: repostImages, status: repostStatus = '' } = originPost || {};
    if (this.isRepost() && (
      _.isEmpty(originPost) || (originPost.postType === 'REPOST' && repostStatus === '')
    )) {
      return null;
    }
    const thumbnail = _.get(shouldRenderRepost ? reposterProfile : profile, 'avatarHashes.tiny');
    const reposterPeerId = _.get(originPost, 'vendorID.peerID');
    return (
      <TouchableWithoutFeedback
        onPress={this.handlePostNavigation}
        pointerEvents="box-none"
      >
        <View style={[styles.wrapper, style]}>
          {shouldRenderRepost && (
            <TouchableWithoutFeedback onPress={this.handlePressAvatar} >
              <View style={styles.repostIndicator}>
                <Feather
                  style={styles.buttonIcon}
                  name="repeat"
                  size={15}
                  color={brandColor}
                />
                <Text style={styles.providerName}>{profileName}</Text>
                <Text style={styles.repostText}>{I18n.t('components.molecules.FeedItem.reposted')}</Text>
              </View>
            </TouchableWithoutFeedback>
          )}
          <View style={styles.contentContainer}>
            <AvatarImage
              style={chatStyles.avatarImage}
              thumbnail={thumbnail}
              onPress={this.handlePressOriginalPostOwner}
            />
            <View style={styles.nonAvatarContainer}>
              {this.renderHeader(name, this.isRepost() ? time : timestamp)}
              {this.renderContentWithListingPreview()}
              {!this.isRepost() ? (
                <FeedImageGrid images={images} onViewImages={onViewImages} />
              ) : status === '' ? (
                <FeedImageGrid images={repostImages} onViewImages={onViewImages} />
              ) : (
                <View style={styles.previewWrapper}>
                  <FeedPreview peerID={reposterPeerId} activityId={activityId} />
                </View>
                  )}
              <FeedItemFooter
                style={styles.footer}
                activityId={shouldRenderRepost ? originActivity.id : item.id}
                item={shouldRenderRepost ? originActivity : item}
                toPost={onComment}
                repost={repost}
                deleteRepost={deleteRepost}
              />
            </View>
          </View>
          {this.renderBottom()}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data || {},
  getActivity: getActivity(state),
  listings: state.listings.data,
});

const mapDispatchToProps = {
  fetchListing,
  deleteFeedItem,
  removeFeedItem,
  fetchGlobalFeedItem,
  blockNode,
  hideToast,
  showToast,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
  null,
  { withRef: true },
)(FeedItem));
