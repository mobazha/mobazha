import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { RefreshControl, Platform, Dimensions } from 'react-native';
import * as _ from 'lodash';
import decode from 'unescape';

import NavBackButton from '../atoms/NavBackButton';
import StoreMoreMenu from '../molecules/StoreMoreMenu';
import ShopInfo from '../molecules/ShopInfo';
import Tabs from '../organism/Tabs';
import ProductModeSelector from '../organism/ProductModeSelector';
import AboutTab from './AboutTab';
import ListingsTab from './ListingsTab';
import FeedTemplate from './feed';
import ParallaxScrollView from './ParallaxScrollView';

import { fetchStream } from '../../reducers/stream';
import { fetchProfile } from '../../reducers/profile';

import { getHeaderImageSource, getLocalHeaderImageSource } from '../../utils/files';
import { navHeightStyle } from '../../utils/navbar';

import { statusbarHeight } from '../../status-bar';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const tabs = [
  {
    label: I18n.t('components.templates.StoreTabs.store'),
    value: 'store',
  },
  {
    label: I18n.t('components.templates.StoreTabs.posts'),
    value: 'posts',
  },
  {
    label: I18n.t('components.templates.StoreTabs.about'),
    value: 'about',
  },
];

class StoreTabs extends PureComponent {
  state = {
    currentTab: 'store',
    mode: 'list',
    counts: 0,
    endReached: false,
  };

  componentDidMount() {
    this.handleRefresh();
  }

  componentDidUpdate(prevProps) {
    if (this.props.peerID !== prevProps.peerID) {
      this.fetchFeeds();
    }
  }

  onChangeMode = (mode) => {
    eventTracker.trackEvent('UserProfile-ChangedStoreLayout', { mode });
    this.setState({ mode });
  }

  setListingsTabRef = (ref) => {
    if (ref) {
      this.listingsTab = ref.wrappedInstance;
    }
  }

  handleRefresh = () => {
    const { peerID, fetchProfile } = this.props;
    this.fetchFeeds();
    fetchProfile({ peerID });
    if (this.listingsTab) {
      this.listingsTab.updateListings();
    }
  }

  fetchFeeds = () => {
    const { peerID, profile, fetchStream } = this.props;
    if (peerID) {
      fetchStream({ filter: 'local', appending: false, peerID: peerID || profile.peerID });
    } else {
      fetchStream({ filter: 'local', appending: false, peerID: '' });
    }
  }

  changeListingCount = (counts) => {
    this.setState({
      counts,
    });
  };

  handleBack = () => this.props.navigation.goBack()

  handleTabChange = (tabVal) => {
    const { externalStore } = this.props;
    eventTracker.trackEvent('UserProfile-ViewedProfileDetails', { tabVal, externalStore });
    this.setState({ currentTab: tabVal });
  }

  handleScrollEndDrag = (evt) => {
    const {
      nativeEvent: {
        layoutMeasurement: { height: layoutHeight },
        contentSize: { height: contentHeight },
      },
    } = evt;
    const targetOffset = _.get(
      evt,
      Platform.OS === 'ios' ? 'nativeEvent.targetContentOffset.y' : 'nativeEvent.contentOffset.y',
    );
    if (contentHeight <= targetOffset + layoutHeight + 80) {
      this.setState(
        { endReached: true },
        () => {
          this.setState({ endReached: false });
        },
      );
    }
  }

  scrollCallBacks = Platform.select({
    ios: { onScrollEndDrag: this.handleScrollEndDrag },
    android: {
      onMomentumScrollEnd: this.handleScrollEndDrag,
      onScrollEndDrag: this.handleScrollEndDrag,
    },
  });

  renderTabContent() {
    const { profile, peerID, externalStore, toListingDetails } = this.props;
    const { currentTab, mode, endReached } = this.state;
    switch (currentTab) {
      case 'store':
        return (
          <ListingsTab
            onRef={this.setListingsTabRef}
            mode={mode}
            endReached={endReached}
            peerID={peerID}
            onPress={toListingDetails}
            externalStore={externalStore}
            onListingCountChange={this.changeListingCount}
          />
        );
      case 'about':
        return <AboutTab profile={profile.data} />;
      default:
        return (
          <FeedTemplate
            name={decode(_.get(profile, 'data.name'))}
            peerID={peerID}
            externalStore={externalStore}
          />
        );
    }
  }

  renderStickyPart = () => {
    const { currentTab, mode, counts } = this.state;
    if (currentTab === 'store' && counts > 0) {
      return (
        <ProductModeSelector
          mode={mode}
          counts={counts}
          onChange={this.onChangeMode}
        />
      );
    }
    return null;
  };

  render() {
    const {
      profile, peerID, externalStore, onMore,
    } = this.props;
    const { currentTab } = this.state;

    const cover = _.get(profile.data, 'headerHashes.medium');
    const navBarHeight = navHeightStyle.height + statusbarHeight;

    return (
      <ParallaxScrollView
        windowHeight={SCREEN_WIDTH / 3 + navBarHeight}
        backgroundSource={externalStore ? getHeaderImageSource(cover) : getLocalHeaderImageSource(cover)}
        leftIcon={NavBackButton}
        rightIcon={props => <StoreMoreMenu onMore={onMore} {...props} />}
        leftIconOnPress={this.handleBack}
        rightIconOnPress={onMore}
        navBarColor="white"
        navBarTitleColor="black"
        navBarTitle={decode(_.get(profile.data, 'name'))}
        refreshControl={<RefreshControl refreshing={profile.loading} onRefresh={this.handleRefresh} />}
        stickyHeaderIndices={[1]}
        {...this.scrollCallBacks}
        contentContainerStyle={{ paddingTop: SCREEN_WIDTH / 3 - 32 + (Platform.OS === 'android' && statusbarHeight) }}
      >
        <ShopInfo
          peerID={peerID}
          profile={profile.data}
          externalStore={externalStore}
        />
        <Tabs
          currentTab={currentTab}
          tabs={tabs}
          onChange={this.handleTabChange}
        />
        {!peerID && this.renderStickyPart()}
        {this.renderTabContent()}
      </ParallaxScrollView>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
});

const mapDispatchToProps = { fetchStream, fetchProfile };

export default connect(mapStateToProps, mapDispatchToProps)(StoreTabs);
