import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { withNavigation } from 'react-navigation';
import { ActivityIndicator, FlatList, Text, View } from 'react-native';
import ImageViewer from 'react-native-image-zoom-viewer';
import moment from 'moment';

import HollowButton from '../atoms/HollowButton';
import FeedItem from '../molecules/FeedItem';
import ModalBackButton from '../atoms/ModalBackButton';
import { OBDarkModal } from '../templates/OBModal';
import RepostTemplate from '../templates/SocialPostTemplate/RepostTemplate';
import ReportTemplate from './ReportTemplate';

import { actions, fetchStream, resetFeedList } from '../../reducers/stream';
import { reportProfile } from '../../api/profile';
import { secondaryTextColor, foregroundColor } from '../commonColors';
import { getActivity, getFilteredList } from '../../selectors/stream';
import { eventTracker } from '../../utils/EventTracker';
import { STREAM_FEED_PAGE_SIZE } from '../../config/stream';

import {I18n} from '../../langs/I18n';

const styles = {
  emptyHeader: {
    height: 0,
  },
  loading: {
    marginTop: 30,
  },
  noPosts: {
    marginTop: 30,
    textAlign: 'center',
    fontSize: 18,
  },
  wrapper: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: foregroundColor,
    paddingVertical: 43,
    borderTopWidth: 0,
  },
  mainText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#8a8a8f',
    marginBottom: 2,
  },
  description: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
    marginBottom: 10,
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
    top: 100,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

class Feed extends PureComponent {
  state = {
    imagesForModal: [], reported: false, reportingID: '',
  };

  componentDidMount() {
    this.updateFeed();
  }

  componentDidUpdate(prevProps) {
    const { peerID } = this.props;
    if (peerID !== prevProps.peerID) {
      this.updateFeed();
    }
  }

  setRepostTemplateRef = (ref) => { this.repostTemplate = ref && ref.wrappedInstance; }

  setReportTemplateRef = (component) => { this.reportTemplate = component; }

  getFeedList = () => {
    const { peerID, profile } = this.props;
    const filter = peerID === profile.peerID ? 'local' : 'global';
    const feedList = this.props.getFilteredList(filter, 'recent', peerID || profile.peerID);
    return feedList;
  }

  updateFeed() {
    const { peerID, profile, fetchStream, resetFeedList } = this.props;
    resetFeedList('local');
    fetchStream({ filter: 'local', appending: true, peerID: peerID || profile.peerID });
  }

  handlePress = (activityId, tab = 'comment', showKeyboard = false) => {
    const { navigation } = this.props;
    eventTracker.trackEvent('Social-ViewedPostDetails');
    navigation.push('FeedDetail', {
      activityId, tab, showKeyboard,
    });
  };

  handleViewImages = (images) => {
    this.setState({ imagesForModal: images });
  }

  handleRepost = (reference, activityId) => {
    this.repostTemplate.repost(reference, activityId);
  }

  handleDeleteRepost = (reference, activityId, reactionId, originalActivityId) => {
    this.repostTemplate.deleteRepost(reference, activityId, reactionId, originalActivityId);
  }

  handleLoad = () => {
    const { currentStatus, fetchStream, allFetched, peerID } = this.props;
    const feedList = this.getFeedList();
    if (currentStatus !== actions.fetchStream && !allFetched && feedList.length >= STREAM_FEED_PAGE_SIZE) {
      fetchStream({ filter: 'local', peerID, appending: true });
    }
  }

  startReport = (peerID) => {
    this.setState({ reportingID: peerID });
    if (this.reportTemplate) {
      this.reportTemplate.startStep();
    }
  }

  reportUser = (reason, option) => {
    const { reportingID: peerID } = this.state;
    reportProfile(peerID, `${option} - ${reason}`, 'profile', 'post');
    this.showTooltip();
  }

  showTooltip() {
    this.setState({ reported: true }, () => {
      setTimeout(() => { this.setState({ reported: false }); }, 2000);
    });
  }

  createPost = () => {
    const { navigation } = this.props;
    navigation.navigate('NewFeed');
  };

  handleClose = () => {
    this.setState({ imagesForModal: [] });
  };

  keyExtractor = (item) => {
    const { getActivity } = this.props;
    const { updated } = getActivity(item);
    return updated ? `local_feed_item_${item}_${moment(updated).toISOString()}` : `local_feed_item_${item}`;
  }

  renderItem = ({ item, index }) => (
    <FeedItem
      activityId={item}
      onPress={this.handlePress}
      onComment={this.handlePress}
      isFirst={index === 0}
      onViewImages={this.handleViewImages}
      reportUser={this.startReport}
      repost={this.handleRepost}
      deleteRepost={this.handleDeleteRepost}
    />
  );

  renderFooter = () => {
    const { currentStatus } = this.props;
    const feedList = this.getFeedList();
    if (currentStatus === actions.fetchStream && feedList.length > 0) {
      return (
        <View style={styles.loadingWrapper}>
          <ActivityIndicator size="large" />
        </View>
      );
    }
    return false;
  }

  renderEmptyState = () => {
    const { peerID, profile, name, currentStatus } = this.props;
    if (currentStatus === actions.fetchStream) {
      return <ActivityIndicator style={styles.loading} size="large" color={secondaryTextColor} />;
    }

    if (profile.peerID !== peerID && peerID) {
      return (
        <View style={styles.wrapper}>
          <Text style={styles.description}>{name} {I18n.t('components.templates.feed.not_post')}</Text>
        </View>
      );
    }

    return (
      <View style={styles.wrapper}>
        <Text style={styles.mainText}>{I18n.t('components.templates.feed.post_hint1')} </Text>
        <Text style={styles.description}>{I18n.t('components.templates.feed.post_hint2')}</Text>
        <HollowButton title={I18n.t('components.templates.feed.Create_post')} onPress={this.createPost} />
      </View>
    );
  }

  render() {
    const { refreshControl, loading } = this.props;
    const feedList = this.getFeedList();
    const { imagesForModal, reported } = this.state;
    return (
      <View style={{ flex: 1 }}>
        <FlatList
          data={feedList}
          extraData={loading}
          keyExtractor={this.keyExtractor}
          renderItem={this.renderItem}
          onEndReached={this.handleLoad}
          onEndReachedThreshold={0.9}
          refreshControl={refreshControl}
          ListFooterComponent={this.renderFooter}
          ListEmptyComponent={this.renderEmptyState}
        />
        {reported && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{I18n.t('components.templates.feed.reported')}</Text>
          </View>
        )}
        <ReportTemplate
          ref={this.setReportTemplateRef}
          submit={this.reportUser}
        />
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
        <RepostTemplate ref={this.setRepostTemplateRef} />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
  getFilteredList: getFilteredList(state),
  currentStatus: state.stream.currentStatus,
  allFetched: state.stream.localAllFetched,
  getActivity: getActivity(state),
});

const mapDispatchToProps = {
  fetchStream, resetFeedList,
};

export default withNavigation(connect(mapStateToProps, mapDispatchToProps)(Feed));
