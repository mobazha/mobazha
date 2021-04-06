import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, FlatList, ActivityIndicator, Text } from 'react-native';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import decode from 'unescape';

import ProductListItem from '../molecules/ProductListItem';
import SearchFilterHeader from '../organism/SearchFilterHeader';
import { foregroundColor, secondaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  resultWrapper: {
    backgroundColor: foregroundColor,
    flex: 1,
  },
  listMode: {
    flexDirection: 'column',
  },
  activityIndicator: {
    paddingBottom: 10,
    marginTop: 10,
  },
  emptyWrapper: {
    flex: 1,
    backgroundColor: '#ffffff',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 185,
  },
  emptyIcon: {
    marginBottom: 10,
  },
  emptyText: {
    color: '#8a8a8f',
    fontSize: 15,
    width: 294,
    textAlign: 'center',
  },
};

class SearchResults extends PureComponent {
  state = { isInitialEmpty: true }

  componentDidUpdate(prevProps, prevState) {
    if (this.state.isInitialEmpty) {
      if (this.props.loading && !prevProps.loading) {
        this.setState({ isInitialEmpty: false });
      }
    }
  }

  handleEndReached = () => {
    this.props.load();
  }

  renderListFooter = () => {
    const { hasMore } = this.props;
    return (
      hasMore && (
        <ActivityIndicator
          style={styles.activityIndicator}
          size="large"
          color={secondaryTextColor}
        />
      )
    );
  };

  renderLoadingState = () => (
    <View style={styles.emptyWrapper}>
      <ActivityIndicator style={styles.activityIndicator} size="large" color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('components.templates.SearchResults.loading_results')}</Text>
    </View>
  );

  renderEmptyState = () => (
    <View style={styles.emptyWrapper}>
      <Ionicons style={styles.emptyIcon} name="md-search" size={50} color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('components.templates.SearchResults.no_found')}</Text>
    </View>
  );

  renderInitialEmptyState = () => (
    <View style={styles.emptyWrapper}>
      <Ionicons style={styles.emptyIcon} name="md-search" size={50} color="#8a8a8f" />
      <Text style={styles.emptyText}>{this.props.initialEmptyTip}</Text>
    </View>
  );

  renderListItem = ({ item }) => {
    const { data, relationships } = item;
    const {
      title, freeShipping, thumbnail, averageRating, ratingCount, price, slug, hash,
    } = data;
    const { data: vendorProfile } = relationships.vendor;

    return (
      <ProductListItem
        title={decode(title)}
        freeShipping={freeShipping}
        slug={slug}
        hash={hash}
        thumbnail={thumbnail.small}
        averageRating={averageRating}
        ratingCount={ratingCount}
        currencyCode={price.currency?price.currency.code:price.currencyCode}
        amount={price.amount}
        peerID={vendorProfile?vendorProfile.peerID:""}
        fetchedProfile={vendorProfile}
        onPress={this.props.toListingDetails}
      />
    );
  }

  renderListHeader = () => {
    const { results, total } = this.props;
    const finalResult = _.filter(results, o => !_.isEmpty(o));
    return <SearchFilterHeader length={finalResult.length} total={total} />;
  }

  render() {
    const { results, loading, initialEmptyTip } = this.props;
    const { isInitialEmpty } = this.state;
    const finalResult = _.filter(results, o => !_.isEmpty(o));
    if (loading) {
      return this.renderLoadingState();
    }

    if (isInitialEmpty && initialEmptyTip) {
      return this.renderInitialEmptyState();
    }

    if (finalResult.length === 0) {
      return this.renderEmptyState();
    }

    return (
      <View style={styles.resultWrapper}>
        <FlatList
          numColumns={1}
          keyExtractor={(item, index) => `search_result_${index}`}
          data={finalResult}
          renderItem={this.renderListItem}
          onEndReached={this.handleEndReached}
          onEndReachedThreshold={0.8}
          ListHeaderComponent={this.renderListHeader()}
          ListFooterComponent={this.renderListFooter()}
        />
      </View>
    );
  }
}

const mapStateToProps = ({ search: { loading } }) => ({
  loading,
});

export default connect(mapStateToProps)(SearchResults);
