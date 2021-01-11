import React, { PureComponent } from 'react';
import { FlatList, View } from 'react-native';
import decode from 'unescape';
import _ from 'lodash';

import ProductCard from '../molecules/ProductCard';
import ProductCardLoader from '../atoms/ProductCardLoader';

import {I18n} from '../../langs/I18n';

const styles = {
  compactWrapper: {
    paddingHorizontal: 16,
  },
  wrapper: {
    paddingHorizontal: 0,
  },
  columnStyle: {
    justifyContent: 'space-between',
    marginTop: 8,
  },
};

export default class ProductGrid extends PureComponent {
  keyExtractor = (item, index) => `${this.props.keyPrefix}_${index}`

  renderItem = ({ item, index }) => {
    const { keyPrefix, compact, toListingDetails } = this.props;
    const itemInfo = item.data || item || {};
    return (
      <ProductCard
        compact={compact}
        hash={itemInfo.hash}
        slug={itemInfo.slug}
        title={decode(itemInfo.title)}
        thumbnail={_.get(itemInfo, 'thumbnail.small')}
        averageRating={itemInfo.averageRating}
        ratingCount={itemInfo.ratingCount}
        currencyCode={_.get(itemInfo, 'bigPrice.currencyCode')}
        amount={_.get(itemInfo, 'bigPrice.amount')}
        onPress={toListingDetails}
        key={`${keyPrefix}_${index}`}
      />
    );
  };

  render() {
    const { products, compact } = this.props;
    if (_.isEmpty(products)) {
      return (
        <View style={compact ? styles.compactWrapper : styles.wrapper}>
          <ProductCardLoader compact={compact} />
        </View>
      );
    }
    return (
      <FlatList
        contentContainerStyle={compact ? styles.compactWrapper : styles.wrapper}
        columnWrapperStyle={styles.columnStyle}
        numColumns={compact ? 3 : 2}
        data={products}
        keyExtractor={this.keyExtractor}
        renderItem={this.renderItem}
      />
    );
  }
}
