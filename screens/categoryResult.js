import React, { PureComponent } from 'react';
import { View } from 'react-native';
import { connect } from 'react-redux';
import * as _ from 'lodash';

import SearchResults from '../components/templates/SearchResults';
import { screenWrapper } from '../utils/styles';
import { appendSearch } from '../reducers/search';
import { setCategory, setCategoryKeyword } from '../reducers/appstate';
import SearchHeader from '../components/organism/SearchHeader';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

const styles = {
  header: {
    backgroundColor: 'white',
  },
};

class CategoryResult extends PureComponent {
  state = { keyword: '' }

  componentDidMount() {
    const { navigation, setCategory } = this.props;
    const searchAutoFocus = navigation.getParam('searchAutoFocus');

    if (!searchAutoFocus) {
      setCategory(this.getCurrentCategory());
    }
  }

  getCurrentCategory = () => {
    const { navigation } = this.props;
    const { name } = navigation.getParam('item');
    const sub = navigation.getParam('sub');
    return sub ? `${name},${sub}` : name;
  }

  getInitialEmptyTyip = () => {
    const { navigation } = this.props;
    const { name } = navigation.getParam('item');
    const sub = navigation.getParam('sub');
    return sub ? null : `Search in ${name}`;
  }

  handleGoBack = () => {
    this.props.navigation.goBack();
  };

  toFilter = () => {
    this.props.navigation.navigate('SearchFilter');
  }

  handleGoToListingDetails = (params) => {
    const { navigation } = this.props;
    const { name } = navigation.getParam('item');
    const sub = navigation.getParam('sub');
    eventTracker.trackEvent('Discover-TappedListing-CategoryResult', {
      reference: 'CategoryResult', category: name, subCategory: sub,
    });
    navigation.navigate('Listing', params);
  }

  handleSearch = () => {
    const { navigation } = this.props;
    const { keyword } = this.state;

    const { name } = navigation.getParam('item');
    const sub = navigation.getParam('sub');
    eventTracker.trackEvent('Discover-PerformedSearch-Category', {
      category: name, subCategory: sub,
    });
    this.props.setCategoryKeyword({ category: this.getCurrentCategory(), keyword });
  }

  handleChangeKeyword = (keyword) => {
    this.setState({ keyword });
  }

  handleGoToFilter = () => {
    this.props.navigation.navigate('SearchFilter');
  }

  render() {
    const {
      search_result, more_page, cur_page, total, appendSearch, navigation,
    } = this.props;
    const { keyword } = this.state;
    const searchAutoFocus = navigation.getParam('searchAutoFocus');

    return (
      <View style={screenWrapper.wrapper}>
        <SearchHeader
          contentStyle={styles.header}
          showQRButton={false}
          doSearch={this.handleSearch}
          onChange={this.handleChangeKeyword}
          keyword={keyword}
          navigation={navigation}
          toFilter={!_.isEmpty(search_result) && this.handleGoToFilter}
          onBack={this.handleGoBack}
          autoFocus={searchAutoFocus}
          lightStatus
        />
        <SearchResults
          results={search_result}
          hasMore={more_page}
          curPage={cur_page}
          total={total}
          load={appendSearch}
          toListingDetails={this.handleGoToListingDetails}
          initialEmptyTip={this.getInitialEmptyTyip()}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  search_result: state.search.search_result,
  more_page: state.search.more_page,
  cur_page: state.search.cur_page,
  total: state.search.total,
});

const mapDispatchToProps = {
  setCategory,
  setCategoryKeyword,
  appendSearch,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(CategoryResult);
