import React from 'react';
import { connect } from 'react-redux';
import { withNavigation, NavigationEvents } from 'react-navigation';
import { ActivityIndicator, View, Text, Dimensions, ScrollView, Platform, RefreshControl } from 'react-native';
import ImageViewer from 'react-native-image-zoom-viewer';
import Feather from 'react-native-vector-icons/Feather';
import * as _ from 'lodash';
import uuidv4 from 'uuid/v4';

import ModalBackButton from '../atoms/ModalBackButton';
import HollowButton from '../atoms/HollowButton';
import FeedItem from '../molecules/FeedItem';
import CommentInput from '../organism/CommentInput';
import Tabs from '../organism/Tabs';
import { OBDarkModal } from './OBModal';
import FeedTabContent from './FeedTabContent';
import RepostTemplate from './SocialPostTemplate/RepostTemplate';
import ReportTemplate from './ReportTemplate';

import StreamClient from '../../StreamClient';

import { fetchFeedItems, actions as streamActions } from '../../reducers/stream';
import { commentFeed } from '../../reducers/feed';
import { reportProfile } from '../../api/profile';
import { getActivity } from '../../selectors/stream';
import { formLabelColor } from '../commonColors';

import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const { height } = Dimensions.get('screen');

const tabs = [
  {
    label: 'Comments',
    value: 'comment',
  },
  {
    label: 'Reposts',
    value: 'repost',
  },
  {
    label: 'Likes',
    value: 'like',
  },
];

const styles = {
  wrapper: {
    flex: 1,
    paddingBottom: Platform.OS === 'ios' && height >= 800 ? 20 : 0,
  },
  content: { flex: 1 },
  fullWrapper: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptyText: {
    color: '#8a8a8f',
    fontSize: 15,
    textAlign: 'center',
    marginVertical: 10,
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 120,
    bottom: 80,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

class FeedDetail extends React.PureComponent {
  state = {
    imagesForModal: [],
    currentTab: 'comment',
    loading: false,
    reported: false,
    fetching: true,
    comments: [],
    likes: [],
    reposts: [],
  };

  componentDidMount() {
    this.loadReactions();
  }

  componentDidUpdate(prevProps) {
    const { navigation, currentStatus } = this.props;
    const activityId = navigation.getParam('activityId');
    const prevActivityId = prevProps.navigation.getParam('activityId');
    if (activityId !== prevActivityId) {
      this.loadReactions();
    }
    if (currentStatus !== streamActions.fetchFeedItems && prevProps.currentStatus === streamActions.fetchFeedItems) {
      this.setState({ loading: false });
    }
  }

  setRepostTemplateRef = (ref) => { this.repostTemplate = ref && ref.wrappedInstance; }
  setFeedItemRef = (ref) => { this.feedItem = ref && ref.wrappedInstance; }
  setCommentTabRef = (ref) => { this.commentTab = ref && ref.wrappedInstance; }
  setCommentInputRef = (ref) => { this.commentInput = ref && ref.wrappedInstance; }
  setScrollRef = (ref) => { this.scroll = ref; }
  setReportTemplateRef = (component) => { this.reportTemplate = component; }

  getActivity = () => {
    const { navigation, getActivity } = this.props;
    const activityId = navigation.getParam('activityId');
    return getActivity(activityId);
  }

  loadReactions = async () => {
    const { navigation } = this.props;
    const activityId = navigation.getParam('activityId');
    this.setState({ fetching: true });

    try {
      const [comments, reposts, likes] = await Promise.all([
        StreamClient.getReactions(activityId, 'comment'),
        StreamClient.getReactions(activityId, 'repost'),
        StreamClient.getReactions(activityId, 'like'),
      ]);
      this.setState({
        comments: comments ? comments.results.reverse() : [],
        reposts: reposts ? reposts.results : [],
        likes: likes ? likes.results : [],
        fetching: false,
      });
    } catch (err) {
      if (err === 'STREAM_NOT_INITIALIZED') {
        setTimeout(this.loadReactions, 3000);
      }
    }
  }

  loadReposts = async () => {
    const { navigation } = this.props;
    const activityId = navigation.getParam('activityId');

    try {
      const reposts = await StreamClient.getReactions(activityId, 'repost');
      this.setState({ reposts: reposts ? reposts.results : [] });
    } catch {
    }
  }

  handleNavigationFocus = () => {
    const { navigation } = this.props;
    const { currentTab } = this.state;
    const tab = navigation.getParam('tab');
    const showKeyboard = navigation.getParam('showKeyboard');
    this.setState({ currentTab: currentTab || (tab || 'comment') }, () => {
      setTimeout(() => {
        if (this.commentInput && showKeyboard) {
          this.commentInput.setFocus();
        }
      }, 500);
    });
  }

  handleChangeTab = (tabVal) => {
    switch (tabVal) {
      case 'like':
        eventTracker.trackEvent('Social-ViewedLikes');
        break;
      case 'repost':
        eventTracker.trackEvent('Social-ViewedReposts');
        break;
      default:
        break;
    }
    this.setState({ currentTab: tabVal });
  }

  handleRepost = (reference, activityId) => {
    this.repostTemplate.repost(reference, activityId);
  }

  handleDeleteRepost = (reference, activityId, reactionId, originalActivityId) => {
    this.repostTemplate.deleteRepost(reference, activityId, reactionId, originalActivityId);
  }

  handleViewImages = (images) => {
    eventTracker.trackEvent('Social-ViewedPhotos');
    this.setState({ imagesForModal: images });
  }

  handleComment = () => {
    this.setState({ currentTab: 'comment' }, () => {
      setTimeout(() => {
        if (this.commentInput) {
          this.commentInput.setFocus();
        }
      }, 500);
    });
  }

  handleRefresh = () => {
    const { navigation, fetchFeedItems } = this.props;
    const activityId = navigation.getParam('activityId');
    this.setState({ loading: true });
    fetchFeedItems([activityId]);
    this.loadReactions();
  }

  reportUser = (peerID, slug) => {
    this.setState({ reportingPeerID: peerID, reportingSlug: slug });
    if (this.reportTemplate) {
      this.reportTemplate.startStep();
    }
  }

  submitReport = (reason, option) => {
    const { reportingPeerID: peerID, reportingSlug: slug } = this.state;
    reportProfile(peerID, `${option} - ${reason}`, slug, 'post');
    this.setState({ reported: true }, () => {
      setTimeout(() => { this.setState({ reported: false }); }, 2000);
    });
  }

  handleClose = () => {
    this.setState({ imagesForModal: [] });
  };

  handleCommentSuccess = async () => {
    await this.loadReactions();
    if (this.commentTab) {
      this.commentTab.postSuccess();
    }
  }

  handleShowActionSheet = () => {
    if (this.feedItem) {
      this.feedItem.handleShowActionSheet();
    }
  }

  handleDeleteComment = (reactionId) => {
    const { comments } = this.state;
    this.setState({ comments: comments.filter(cmt => cmt.id !== reactionId) })
  }

  submitComment = (comment) => {
    const item = this.getActivity();
    const uuid = uuidv4();
    const slug = _.get(item, 'object.data.post.slug');
    const peerID = _.get(item, 'object.data.post.vendorID.peerID');
    const { id: activityId } = item;
    eventTracker.trackEvent('Social-CommentedPost');
    this.props.commentFeed({
      uuid,
      comment,
      reference: `${peerID}/${slug}`,
      activityId,
      onSuccess: this.handleCommentSuccess,
    });
    if (this.commentTab) {
      this.commentTab.posting(comment);
    }
    if (this.scroll) {
      this.scroll.scrollToEnd();
    }
  }

  renderDueState = () => (
    <View style={styles.fullWrapper}>
      <Feather name="file-text" size={48} color={formLabelColor} />
      <Text style={styles.emptyText}>{I18n.t('components.templates.FeedDetail.fail_to_load')}</Text>
      <HollowButton title={I18n.t('components.templates.FeedDetail.retry')} onPress={this.handleRetry} />
    </View>
  );

  renderLoadingState = () => (
    <View style={styles.fullWrapper}>
      <ActivityIndicator size="large" color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('components.templates.FeedDetail.Loading')}</Text>
    </View>
  );

  renderTab = () => {
    const { currentTab, comments, likes, reposts, fetching } = this.state;
    switch (currentTab) {
      case 'comment':
        return (
          <FeedTabContent
            ref={this.setCommentTabRef}
            type="comments"
            data={comments}
            fetching={fetching}
            key="comment"
            onDeleteComment={this.handleDeleteComment}
          />
        );
      case 'like':
        return <FeedTabContent type="likes" data={likes} fetching={fetching} key="likes" />;
      case 'repost':
      default:
        return <FeedTabContent type="reposts" data={reposts} fetching={fetching} key="repost" />;
    }
  };

  render() {
    const { navigation, currentStatus } = this.props;
    const { loading } = this.state;
    const activityId = navigation.getParam('activityId');
    const {
      imagesForModal, currentTab, reported,
    } = this.state;

    if (_.isEmpty(this.getActivity())) {
      return this.renderLoadingState();
    }

    return (
      <View style={styles.wrapper}>
        <NavigationEvents
          onDidFocus={this.handleNavigationFocus}
        />
        <ScrollView
          style={styles.content}
          ref={this.setScrollRef}
          refreshControl={
            <RefreshControl
              refreshing={loading && currentStatus === streamActions.fetchFeedItems}
              onRefresh={this.handleRefresh}
            />
          }
        >
          <FeedItem
            key={`${activityId}`}
            activityId={activityId}
            onPress={() => {}}
            onComment={this.handleComment}
            navigation={navigation}
            onViewImages={this.handleViewImages}
            noSeparator
            repost={this.handleRepost}
            reportUser={this.reportUser}
            deleteRepost={this.handleDeleteRepost}
            onRef={this.setFeedItemRef}
            hideMoreButton
          />
          <Tabs
            currentTab={currentTab}
            tabs={tabs}
            onChange={this.handleChangeTab}
          />
          {this.renderTab()}
        </ScrollView>
        {currentTab === 'comment' && (
          <CommentInput
            ref={this.setCommentInputRef}
            submitComment={this.submitComment}
          />
        )}
        {reported && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>
              {I18n.t('components.templates.FeedDetail.reported')}
            </Text>
          </View>
        )}
        <OBDarkModal
          onRequestClose={this.handleClose}
          visible={imagesForModal.length > 0}
          darkContent
        >
          <ImageViewer
            imageUrls={imagesForModal}
            enableSwipeDown
            enablePreload
            onCancel={this.handleClose}
          />
          <ModalBackButton onPress={this.handleClose} />
        </OBDarkModal>
        <RepostTemplate
          ref={this.setRepostTemplateRef}
          onDeleteRepost={this.loadReposts}
          onAddRepost={this.loadReposts}
        />
        <ReportTemplate ref={this.setReportTemplateRef} submit={this.submitReport} />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
  getActivity: getActivity(state),
  currentStatus: state.stream.currentStatus,
});

const mapDispatchToProps = { commentFeed, fetchFeedItems };

export default withNavigation(
  connect(mapStateToProps, mapDispatchToProps, null, { withRef: true })(FeedDetail),
);
