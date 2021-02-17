import React, { PureComponent } from 'react';
import { View, Keyboard } from 'react-native';
import { connect } from 'react-redux';
import { isEmpty } from 'lodash';
import { NavigationEvents } from 'react-navigation';

import SearchHeader from '../components/organism/SearchHeader';
import SearchResults from '../components/templates/SearchResults';
import UserSearchResults from '../components/templates/UserSearchResults';
import RecentSearch from '../components/templates/RecentSearch';
import { screenWrapper } from '../utils/styles';
import Tabs from '../components/organism/Tabs';

import { doSearch, doSearchUser, appendSearch, appendUserSearch, clearFilter } from '../reducers/search';
import { setKeyword, removeKeyword } from '../reducers/appstate';
import { getNavRouteAndParamsFromURL } from '../utils/navigation';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

const tabs = [
  {
    label: I18n.t('screens.searchResult.Listings'),
    value: 'listings',
  },
  {
    label: I18n.t('screens.searchResult.User'),
    value: 'users',
  },
];

class SearchResult extends PureComponent {
  state = {
    keyword: '',
    showRecent: false,
    isFirst: true,
    currentTab: 'listings',
  }

  componentDidMount() {
    const { doSearch, doSearchUser } = this.props;
    doSearch();
    doSearchUser();
  }

  componentDidUpdate(prevProps) {
    const { keyword } = this.props;
    if (keyword !== prevProps.keyword) {
      this.setState({ keyword });
    }
  }

  componentWillUnmount() {
    this.props.clearFilter();
  }

  onNavigationFocus = () => {
    const { navigation, keyword, didSearch } = this.props;
    const isFromTag = navigation.getParam('isFromTag');
    this.setState({ showRecent: !isFromTag && !didSearch, keyword });
    if (!isFromTag && !didSearch && this.searchHeader) {
      this.searchHeader.focus();
    }
  }

  onChangeTab = (tabVal) => {
    eventTracker.trackEvent('Discover-SwitchTab', { tab: tabVal });
    this.setState({ currentTab: tabVal });
  }

  onFocusSearchField = () => {
    this.setState({ showRecent: true });
  }

  onChangeKeyword = (keyword) => {
    if (keyword === '') {
      eventTracker.trackEvent('Discover-ClearedSearch');
    }
    this.setState({ keyword });
  }

  onSelectKeyword = (keyword) => {
    const { setKeyword } = this.props;
    this.setState({ keyword, showRecent: false, isFirst: false });
    setKeyword(keyword);
    Keyboard.dismiss();
  }

  onRemoveKeyword = (keyword) => { this.props.removeKeyword(keyword); }

  setSearchHeaderRef = (ref) => { this.searchHeader = ref; }

  hideRecent = () => {
    const { showRecent, isFirst } = this.state;
    const { navigation } = this.props;
    if (showRecent) {
      Keyboard.dismiss();
      if (isFirst) {
        navigation.goBack();
      } else {
        this.setState({ showRecent: false });
      }
    } else {
      navigation.goBack();
    }
  }

  doSearch = () => {
    const { keyword } = this.state;
    const { navigation, clearFilter } = this.props;
    const navParam = getNavRouteAndParamsFromURL(keyword);
    if (navParam) {
      const { route, params } = navParam;
      this.setState({ showRecent: false, keyword: '' });
      clearFilter();
      navigation.push(route, params);
    } else {
      const { setKeyword } = this.props;
      this.setState({ showRecent: false, isFirst: false });
      eventTracker.trackEvent('Discover-PerformedSearch');
      setKeyword(keyword);
    }
  }

  toFilter = () => {
    const { navigation } = this.props;
    navigation.navigate('SearchFilter');
  }

  toListingDetails = (params) => {
    eventTracker.trackEvent('Discover-TappedListingFromSearch');
    this.props.navigation.push('Listing', params);
  }

  toStoreDetails = (peerID) => {
    eventTracker.trackEvent('Discover-TappedUserFromSearch');
    this.props.navigation.navigate('ExternalStore', { peerID });
  }

  renderTabContents = () => {
    const { currentTab, keyword } = this.state;
    const {
      search_result,
      user_search_result,
      more_page,
      user_more_page,
      cur_page,
      user_cur_page,
      total,
      total_user,
      appendSearch,
      appendUserSearch,
    } = this.props;
    if (currentTab === 'listings') {
      return (
        <SearchResults
          results={search_result}
          keyword={keyword}
          hasMore={more_page}
          curPage={cur_page}
          total={total}
          load={appendSearch}
          toListingDetails={this.toListingDetails}
        />
      );
    }
    return (
      <UserSearchResults
        results={user_search_result}
        keyword={keyword}
        hasMore={user_more_page}
        curPage={user_cur_page}
        total={total_user}
        load={appendUserSearch}
        toStoreDetails={this.toStoreDetails}
      />
    );
  }

  render() {
    const {
      search_result,
      navigation,
    } = this.props;
    const isFromTag = navigation.getParam('isFromTag');
    const {
      showRecent, keyword, currentTab,
    } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <NavigationEvents
          onWillFocus={this.onNavigationFocus}
        />
        <SearchHeader
          ref={this.setSearchHeaderRef}
          showRecent={showRecent}
          showQRButton={false}
          onFocus={this.onFocusSearchField}
          onBack={this.hideRecent}
          doSearch={this.doSearch}
          onChange={this.onChangeKeyword}
          keyword={keyword}
          navigation={navigation}
          autoFocus={!isFromTag}
          toFilter={!isEmpty(search_result) && this.toFilter}
          disableFilter={currentTab === 'users'}
        />
        <Tabs
          currentTab={currentTab}
          tabs={tabs}
          onChange={this.onChangeTab}
        />
        { this.renderTabContents() }
        {showRecent && (
          <RecentSearch
            onSelectKeyword={this.onSelectKeyword}
            onRemoveKeyword={this.onRemoveKeyword}
          />
        )}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  search_result: state.search.search_result,
  user_search_result: state.search.user_search_result,
  more_page: state.search.more_page,
  user_more_page: state.search.user_more_page,
  cur_page: state.search.cur_page,
  user_cur_page: state.search.user_cur_page,
  total: state.search.total,
  total_user: state.search.total_user,
  didSearch: state.search.didSearch,
  keyword: state.appstate.keyword,
});

const mapDispatchToProps = {
  doSearch,
  doSearchUser,
  setKeyword,
  removeKeyword,
  appendSearch,
  appendUserSearch,
  clearFilter,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(SearchResult);
