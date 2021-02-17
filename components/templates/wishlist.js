import React, { PureComponent } from 'react';
import { Text, View, FlatList } from 'react-native';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';

import ProductCard, { styles as cardStyles } from '../molecules/ProductCard';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    marginTop: 10,
    backgroundColor: 'transparent',
  },
  listMode: {
    flexDirection: 'column',
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

export default class WishList extends PureComponent {
  renderEmptyState = () => (
    <View style={styles.emptyWrapper}>
      <Ionicons style={styles.emptyIcon} name="md-heart" size={50} color="#8a8a8f" />
      <Text style={styles.emptyText}>{I18n.t('components.templates.wishlist.wishlist_empty')}</Text>
    </View>
  );

  renderCardItem = ({ item }) => {
    if (_.isEmpty(item)) {
      return null;
    }
    const images = _.get(item, 'listing.item.images');
    return (
      <ProductCard
        hash={item.hash}
        slug={_.get(item, 'listing.slug')}
        title={_.get(item, 'listing.item.title')}
        thumbnail={images && images[0].small}
        averageRating={0}
        ratingCount={0}
        currencyCode={_.get(item, 'listing.item.priceCurrency.code')}
        amount={_.get(item, 'listing.item.bigPrice')}
        peerID={_.get(item, 'listing.vendorID.peerID')}
        onPress={this.props.toListingDetails}
        isWishlist
      />
    );
  }

  render() {
    const { results } = this.props;
    if (results.length === 0) {
      return this.renderEmptyState();
    }

    return (
      <View style={styles.wrapper}>
        <FlatList
          numColumns={2}
          columnWrapperStyle={cardStyles.columnWrapperStyle}
          keyExtractor={(item, index) => `wishlist_${index}`}
          horizontal={false}
          data={results.length % 2 === 1 ? [...results, {}] : results}
          renderItem={this.renderCardItem}
        />
      </View>
    );
  }
}
