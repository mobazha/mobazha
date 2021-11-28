import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Platform, Alert } from 'react-native';
import { NavigationEvents } from 'react-navigation';

import NavPlusButton from '../components/atoms/NavPlusButton';
import TabHeader from '../components/organism/TabHeader';
import Tabs from '../components/organism/Tabs';
import GlobalFeedTemplate from '../components/templates/GlobalFeed';

import { fetchStream, refreshStream, updateSort, STREAM_DEFAULT_FILTER } from '../reducers/stream';
import { setStreamBuildNotificationSeen } from '../reducers/appstate';
import { getFilteredList } from '../selectors/stream';

import {I18n} from '../langs/I18n';

const filterOptions = [
  { label: I18n.t('screens.feed.My_Feed'), value: 'user', tag: 'NEW' },
  { label: I18n.t('screens.feed.Global'), value: 'global' },
];

class Feed extends PureComponent {
  constructor(props) {
    super(props);
    /* This one is to prevent the old setting to block the user's feed */
    let { filter } = props;
    let sort = 'recent';
    if (filter !== 'user' && filter !== 'global') {
      filter = STREAM_DEFAULT_FILTER;
      sort = props[`${filter}Sort`];
    }
    this.state = { filter, sort };

    this.focusedOnce = false;
  }

  setGlobalFeedRef = (ref) => { if (ref) { this.globalFeed = ref.wrappedInstance; } }

  initStreamFeed = () => {
    const { fetchStream } = this.props;
    const { filter, sort } = this.state;
    if (filter !== 'user' && filter !== 'global') {
      fetchStream({ filter: STREAM_DEFAULT_FILTER, sort, appending: true });
    } else {
      fetchStream({ filter, sort, appending: true });
    }
  }

  refreshStreamFeed = () => {
    const { filter, sort, refreshStream } = this.props;
    refreshStream({ filter, sort, appending: false });
  }

  handleFocus = () => {
    const { streamBuildNotificationSeen, setStreamBuildNotificationSeen } = this.props;
    if (!streamBuildNotificationSeen) {
      // Alert.alert(I18n.t('screens.feed.New_features'), I18n.t('screens.feed.feature_description'));
      setStreamBuildNotificationSeen(true);
    }

    if (!this.focusedOnce) {
      this.initStreamFeed();
      this.focusedOnce = true;
    }
  }

  handleChangeFilter = (filter) => {
    const sort = this.props[`${filter}Sort`];
    this.setState({ filter, sort }, () => {
      const { fetchStream } = this.props;
      fetchStream({ filter, sort, appending: true });
    });
  }

  handleChangeSort = (sort) => {
    const { filter } = this.state;
    this.setState({ sort }, () => {
      if (this.globalFeed) {
        const { updateSort } = this.props;
        updateSort({ filter, sort });
      }
    });
  }

  toNewFeed = () => {
    this.props.navigation.navigate('NewFeed');
  }

  render() {
    const { navigation, getFilteredList } = this.props;
    const { filter, sort } = this.state;
    const feedList = getFilteredList(filter, sort);
    return (
      <View style={{ flex: 1, backgroundColor: 'white' }}>
        <NavigationEvents onDidFocus={this.handleFocus} />
        <TabHeader
          left={<NavPlusButton />}
          onLeft={this.toNewFeed}
          title={I18n.t('screens.feed.Social')}
          navigation={navigation}
        />
        {/* { Platform.OS === 'android' && ( */}
          <Tabs
            currentTab={filter}
            tabs={filterOptions}
            onChange={this.handleChangeFilter}
          />
        {/* )} */}
        <GlobalFeedTemplate
          filter={filter}
          sort={sort}
          feedList={feedList}
          onRef={this.setGlobalFeedRef}
          onChangeSortOption={this.handleChangeSort}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  filter: state.stream.filter || STREAM_DEFAULT_FILTER,
  globalSort: state.stream.globalSort || 'recent',
  userSort: state.stream.userSort || 'recent',
  getFilteredList: getFilteredList(state),
  streamBuildNotificationSeen: state.appstate.streamBuildNotificationSeen,
});

const mapDispatchToProps = {
  fetchStream, refreshStream, updateSort, setStreamBuildNotificationSeen,
};

export default connect(mapStateToProps, mapDispatchToProps)(Feed);
