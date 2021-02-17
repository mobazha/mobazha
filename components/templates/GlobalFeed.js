import React from 'react';
import { connect } from 'react-redux';
import { withNavigation, FlatList } from 'react-navigation';
import { ActivityIndicator, Text, View, RefreshControl, Image } from 'react-native';
import ImageViewer from 'react-native-image-zoom-viewer';
import Ionicons from 'react-native-vector-icons/Ionicons';
import * as _ from 'lodash';
import moment from 'moment';

import HollowButton from '../atoms/HollowButton';
import ModalBackButton from '../atoms/ModalBackButton';
import OptionSelector from '../organism/OptionSelector';
import FeedItem from '../molecules/FeedItem';
import { OBDarkModal } from '../templates/OBModal';
import RepostTemplate from './SocialPostTemplate/RepostTemplate';
import ReportTemplate from './ReportTemplate';

import { secondaryTextColor, foregroundColor, primaryTextColor, borderColor } from '../commonColors';
import { reportProfile } from '../../api/profile';
import { actions as streamActions, fetchStream, refreshStream } from '../../reducers/stream';
import { getActivity } from '../../selectors/stream';
import { eventTracker } from '../../utils/EventTracker';

import PersonAddImage from '../../assets/icons/person_add.png';

import {I18n} from '../../langs/I18n';

const sortOptions = [
  { label: I18n.t('components.templates.GlobalFeed.Trending'), value: 'trending' },
  { label: I18n.t('components.templates.GlobalFeed.Most_Recent'), value: 'recent' },
];

const styles = {
  emptyHeader: {
    height: 15,
  },
  emptySearchIcon: {
    marginBottom: 8,
  },
  emptyIcon: {
    width: 64,
    height: 64,
    marginBottom: 8,
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
    borderColor,
  },
  mainText: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    marginBottom: 6,
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
  loadingWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 10,
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
    bottom: 60,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

class Feed extends React.Component {
  state = {
    imagesForModal: [],
    reportingPeerID: '',
    reported: false,
  }

  pauseFetchStream = false;

  shouldComponentUpdate(nextProps, nextState) {
    const { feedList, currentStatus } = this.props;
    const { imagesForModal: currentImages } = this.state;
    const { imagesForModal: nextImages } = nextState;
    const { feedList: nextList, currentStatus: nextStatus } = nextProps;
    if (currentStatus !== nextStatus ||
      !_.isEqual(feedList, nextList) ||
      !_.isEqual(currentImages, nextImages)) {
      return true;
    }
    return false;
  }

  setRepostTemplateRef = (ref) => { this.repostTemplate = ref && ref.wrappedInstance; }
  setReportTemplateRef = (component) => { this.reportTemplate = component; }
  setListRef = (ref) => { this.feedList = ref; }

  handleLoad = () => {
    const {
      currentStatus, fetchStream, filter, sort, allFetched, hashtag, feedList,
    } = this.props;
    if (feedList.length >= 6 && currentStatus !== streamActions.fetchStream &&
      currentStatus !== streamActions.updateSort &&
      !allFetched &&
      !this.pauseFetchStream) {
      this.pauseFetchStream = true;
      fetchStream({ filter, sort, hashtag, appending: true });
      const fetchTimer = setTimeout(() => {
        this.pauseFetchStream = false;
        clearTimeout(fetchTimer);
      }, 500);
    }
  }

  handlePress = (activityId, tab = 'comment', showKeyboard = false) => {
    const { navigation } = this.props;
    eventTracker.trackEvent('Social-ViewedPostDetails');
    navigation.navigate('FeedDetail', { activityId, tab, showKeyboard });
  };

  handleRefresh = () => {
    const { filter, sort, refreshStream } = this.props;
    refreshStream({ filter, sort, appending: false });
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

  createPost = () => {
    const { navigation } = this.props;
    navigation.navigate({ routeName: 'NewFeed', params: { fromGlobalFeed: true } });
  };

  handleClose = () => {
    this.setState({ imagesForModal: [] });
  };

  reportUser = (peerID) => {
    this.setState({ reportingPeerID: peerID });
    if (this.reportTemplate) {
      this.reportTemplate.startStep();
    }
  }

  submitReport = (reason, option) => {
    const { reportingPeerID: peerID } = this.state;
    reportProfile(peerID, `${option} - ${reason}`);
    this.setState({ reported: true }, () => {
      setTimeout(() => { this.setState({ reported: false }); }, 2000);
    });
  }

  keyExtractor = (item) => {
    const { filter, getActivity } = this.props;
    const { updated } = getActivity(item);
    return updated ? `${filter}_feed_item_${item}_${moment(updated).toISOString()}` : `${filter}_feed_item_${item}`;
  }

  renderItem = ({ item, index }) => (
    <FeedItem
      activityId={item}
      onPress={this.handlePress}
      onComment={this.handlePress}
      isFirst={index === 0}
      onViewImages={this.handleViewImages}
      reportUser={this.reportUser}
      repost={this.handleRepost}
      deleteRepost={this.handleDeleteRepost}
    />
  );

  renderFooter = () => {
    const { currentStatus, feedList } = this.props;
    if (currentStatus === streamActions.fetchStream && feedList.length > 0) {
      return (
        <View style={styles.loadingWrapper}>
          <ActivityIndicator size="large" color={secondaryTextColor} />
        </View>
      );
    }
    return false;
  }

  renderEmptyState = () => {
    const { currentStatus, allFetched, filter } = this.props;
    if ((currentStatus === streamActions.fetchStream || !allFetched) && currentStatus !== streamActions.finishedFetchRequest) {
      return (
        <React.Fragment>
          <ActivityIndicator style={styles.loading} size="large" color={secondaryTextColor} />
        </React.Fragment>
      );
    }
    if (filter === 'user') {
      return (
        <View style={styles.wrapper}>
          <Image source={PersonAddImage} style={styles.emptyIcon} />
          <Text style={styles.description}>{I18n.t('components.templates.GlobalFeed.customise_feed')}</Text>
        </View>
      );
    }
    if (filter === 'hashtag') {
      return (
        <View style={styles.wrapper}>
          <Ionicons style={styles.emptySearchIcon} name="md-search" size={50} color="#8a8a8f" />
          <Text style={styles.description}>{I18n.t('components.templates.GlobalFeed.not_found')}</Text>
        </View>
      );
    }
    return (
      <View style={styles.wrapper}>
        <Image source={PersonAddImage} style={styles.emptyIcon} />
        <Text style={styles.description}>{I18n.t('components.templates.GlobalFeed.share_with_community')}</Text>
        <HollowButton title="Create post" onPress={this.createPost} />
      </View>
    );
  }

  render() {
    const { feedList, sort, onChangeSortOption, currentStatus } = this.props;
    const { imagesForModal, reported } = this.state;
    return (
      <View style={{ flex: 1 }}>
        <FlatList
          ref={this.setListRef}
          data={feedList}
          keyExtractor={this.keyExtractor}
          renderItem={this.renderItem}
          refreshControl={
            <RefreshControl refreshing={currentStatus === streamActions.refreshStream} onRefresh={this.handleRefresh} />
          }
          onEndReachedThreshold={0.9}
          onEndReached={this.handleLoad}
          ListHeaderComponent={<OptionSelector currentOption={sort} options={sortOptions} onChange={onChangeSortOption} />}
          ListEmptyComponent={this.renderEmptyState}
          ListFooterComponent={this.renderFooter}
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
        {reported && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>
              {I18n.t('components.templates.GlobalFeed.reported')}
            </Text>
          </View>
        )}
        <RepostTemplate ref={this.setRepostTemplateRef} />
        <ReportTemplate ref={this.setReportTemplateRef} submit={this.submitReport} />
      </View>
    );
  }
}

const mapStateToProps = (state, ownProps) => {
  const { filter } = ownProps;
  const { allFetched } = state.stream[`${filter}Feeds`];
  return {
    getActivity: getActivity(state),
    allFetched,
    currentStatus: state.stream.currentStatus,
  };
};

const mapDispatchToProps = { fetchStream, refreshStream };

export default withNavigation(
  connect(mapStateToProps, mapDispatchToProps, null, { withRef: true })(Feed),
);
