import React, { PureComponent } from 'react';
import { View, FlatList, ActivityIndicator, Text } from 'react-native';
import decode from 'unescape';
import { connect } from 'react-redux';
import * as _ from 'lodash';

import ProductCard, { styles as cardStyles } from '../molecules/ProductCard';
import { getListingResult } from '../../api/search';

import { secondaryTextColor } from '../commonColors';
import ProductModeSelector from '../organism/ProductModeSelector';
import ProductListItem from '../molecules/ProductListItem';

import {I18n} from '../../langs/I18n';

const styles = {
  activityIndicator: {
    paddingBottom: 10,
  },
  emptyWrapper: {
    flex: 1,
    backgroundColor: '#ffffff',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 185,
  },
  emptyText: {
    color: '#8a8a8f',
    fontSize: 15,
    width: 294,
    textAlign: 'center',
  },
};

class InfiniteProducts extends PureComponent {
  constructor(props) {
    super(props);

    this.state = {
      searchResults: [],
      cur_page: 0,
      more_page: true,
      total: 0,
      isLoading: false,
      mode: 'list',
    };

    this.loadMoreIndicator = (
      <ActivityIndicator
        style={styles.activityIndicator}
        size="large"
        color={secondaryTextColor}
      />
    );
  }

  UNSAFE_componentWillMount() {
    this.doInitialSearch();
  }

  doInitialSearch() {
    const { queryString } = this.props;
    this.setState({
      isLoading: true,
    });
    getListingResult(queryString, this.state.cur_page)
      .then((result) => {
        this.setState({
          searchResults: result.results.results,
          cur_page: 0,
          more_page: result.results.morePages,
          total: result.results.total,
          isLoading: false,
        });
      });
  }

  load = () => {
    const { queryString } = this.props;
    const {
      searchResults, more_page, cur_page, isLoading,
    } = this.state;
    if (more_page && !isLoading) {
      this.setState({
        isLoading: true,
      });

      getListingResult(queryString, cur_page + 1)
        .then((result) => {
          this.setState({
            searchResults: searchResults.concat(result.results.results),
            cur_page: cur_page + 1,
            more_page: result.results.morePages,
            total: result.results.total,
            isLoading: false,
          });
        });
    }
  }

  handleChangeMode = (mode) => {
    this.setState({ mode });
  }

  filterBlocked = (listingInfo) => {
    const { blockedNodes } = this.props;
    return !blockedNodes.includes(_.get(listingInfo, 'relationships.vendor.data.peerID'));
  }

  renderLoadingState = () => (
    <View style={styles.emptyWrapper}>
      <ActivityIndicator style={styles.activityIndicator} size="large" color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('components.templates.InfiniteProducts.loading_listings')}</Text>
    </View>
  );

  renderCard = ({ item }) => {
    if (_.isEmpty(item)) {
      return null;
    }

    const { toListingDetails } = this.props;
    const { data = {}, relationships } = item;
    return (
      <ProductCard
        hash={data.hash}
        slug={data.slug}
        title={decode(data.title)}
        thumbnail={_.get(data, 'thumbnail.small')}
        averageRating={data.averageRating}
        ratingCount={data.ratingCount}
        currencyCode={_.get(data, 'bigPrice.currencyCode', '')}
        amount={_.get(data, 'bigPrice.amount', 0)}
        peerID={_.get(relationships, 'vendor.data.peerID')}
        onPress={toListingDetails}
      />
    );
  }

  renderListItem = ({ item }) => {
    if (_.isEmpty(item)) {
      return null;
    }

    const { toListingDetails } = this.props;
    const { data = {}, relationships } = item;
    return (
      <ProductListItem
        hash={data.hash}
        title={decode(data.title)}
        freeShipping={data.freeShipping}
        slug={data.slug}
        thumbnail={_.get(data, 'thumbnail.small')}
        averageRating={data.averageRating}
        ratingCount={data.ratingCount}
        currencyCode={_.get(data, 'bigPrice.currencyCode', '')}
        amount={_.get(data, 'bigPrice.amount', 0)}
        peerID={_.get(relationships, 'vendor.data.peerID')}
        onPress={toListingDetails}
      />
    );
  }

  render() {
    const { showInitialLoading } = this.props;
    const {
      searchResults = [], more_page, total, mode, isLoading,
    } = this.state;

    const filteredResults = searchResults.filter(this.filterBlocked);
    const initialLoading = isLoading && showInitialLoading && total === 0;

    return (
      <View style={{ flex: 1 }}>
        {!!total && (
          <ProductModeSelector
            mode={mode}
            counts={total}
            onChange={this.handleChangeMode}
          />
        )}
        {initialLoading && this.renderLoadingState()}
        {!initialLoading && mode === 'card' && (
          <FlatList
            numColumns={2}
            columnWrapperStyle={cardStyles.columnWrapperStyle}
            keyExtractor={(val, index) => (`infinite_card_${index}`)}
            horizontal={false}
            data={filteredResults}
            renderItem={this.renderCard}
            ListFooterComponent={more_page && this.loadMoreIndicator}
            onEndReached={this.load}
          />
        )}
        {!initialLoading && mode === 'list' && (
          <FlatList
            horizontal={false}
            data={filteredResults}
            keyExtractor={(val, index) => (`infinite_list_${index}`)}
            renderItem={this.renderListItem}
            onEndReached={this.load}
            onEndReachedThreshold={0.8}
            ListFooterComponent={more_page && this.loadMoreIndicator}
          />
        )}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  blockedNodes: state.settings.blockedNodes,
});

export default connect(mapStateToProps, null, null, { withRef: true })(InfiniteProducts);
