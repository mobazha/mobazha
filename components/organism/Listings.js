import React, { PureComponent } from 'react';
import { View, FlatList, Dimensions } from 'react-native';
import { filter, isEmpty, get } from 'lodash';
import decode from 'unescape';
import * as _ from 'lodash';

import ProductCard, { styles as cardStyles } from '../molecules/ProductCard';
import ProductListItem from '../molecules/ProductListItem';

import { foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const listItemHeight = (Dimensions.get('screen').width / 3) + 1;

const styles = {
  resultWrapper: {
    backgroundColor: foregroundColor,
  },
  listMode: {
    flexDirection: 'column',
  },
};

export default class Listings extends PureComponent {
  onEndReached = () => {
    const { infinite, load } = this.props;
    if (infinite) {
      load();
    }
  }

  keyExtractor = (item, index) => `search_result_${index}`

  renderCardItem = ({ item }) => {
    if (_.isEmpty(item)) {
      return null;
    }

    const { peerID, onPress, externalStore } = this.props;
    const { slug, hash, title } = item;
    const priceProps = {
      currencyCode: get(item, externalStore ? 'bigPrice.currencyCode' : 'price.currency.code', ''),
      amount: get(item, externalStore ? 'bigPrice.amount' : 'price.amount', 0),
    };

    return (
      <ProductCard
        slug={slug}
        hash={hash}
        title={decode(title)}
        thumbnail={get(item, 'thumbnail.small')}
        averageRating={get(item, 'averageRating', 0)}
        ratingCount={get(item, 'ratingCount', 0)}
        peerID={peerID}
        onPress={onPress}
        {...priceProps}
      />
    );
  }

  renderListItem = ({ item }) => {
    const {
      peerID, onPress, externalStore,
    } = this.props;
    const {
      title, freeShipping, slug, hash,
    } = item;
    const priceProps = {
      currencyCode: get(item, externalStore ? 'bigPrice.currencyCode' : 'price.currency.code', ''),
      amount: get(item, externalStore ? 'bigPrice.amount' : 'price.amount', 0),
    };
    return (
      <ProductListItem
        hash={hash}
        peerID={peerID}
        slug={slug}
        title={decode(title)}
        freeShipping={freeShipping}
        thumbnail={get(item, 'thumbnail.small')}
        averageRating={get(item, 'averageRating', 0)}
        ratingCount={get(item, 'ratingCount', 0)}
        currencyCode={get(item, 'price.currency.code', '')}
        amount={get(item, 'price.amount', 0)}
        onPress={onPress}
        hideSellerName
        {...priceProps}
      />
    );
  }

  render() {
    const {
      results, mode, status,
    } = this.props;
    const finalResult = filter(results, o => !isEmpty(o));
    return (
      <View style={styles.resultWrapper}>
        {mode === 'card' && (
          <FlatList
            style={{
              height:
                (cardStyles.wrapper.height + cardStyles.columnWrapperStyle.paddingBottom) *
                Math.ceil(finalResult.length / 2),
            }}
            numColumns={2}
            columnWrapperStyle={cardStyles.columnWrapperStyle}
            keyExtractor={this.keyExtractor}
            horizontal={false}
            data={
              finalResult.length % 2 === 0 ? finalResult.reverse() : [...finalResult.reverse(), {}]
            }
            renderItem={this.renderCardItem}
            onEndReached={this.onEndReached}
            onEndReachedThreshold={0.8}
            key={`${status}_card_view`}
          />
        )}
        {mode === 'list' && (
          <FlatList
            style={{ height: listItemHeight * finalResult.length }}
            horizontal={false}
            data={finalResult.reverse()}
            keyExtractor={this.keyExtractor}
            renderItem={this.renderListItem}
            onEndReached={this.onEndReached}
            onEndReachedThreshold={0.8}
            key={`${status}_list_view`}
          />
        )}
      </View>
    );
  }
}
