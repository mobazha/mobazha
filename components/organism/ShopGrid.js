import React from 'react';
import { Dimensions, FlatList, View } from 'react-native';
import * as _ from 'lodash';

import ShopCard from '../molecules/ShopCard';
import ShopGridLoader from '../atoms/ShopGridLoader';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const CARD_WIDTH = (SCREEN_WIDTH - 40) / 2;
const CARD_HEIGHT = CARD_WIDTH / 2;

const styles = {
  card: {
    width: CARD_WIDTH,
    height: CARD_HEIGHT,
  },
  columnWrapperStyle: {
    paddingHorizontal: 16,
    justifyContent: 'space-between',
    marginTop: 16,
  },
  mainWrapper: {
    position: 'relative',
  },
  contentLoaderWrapper: {
    position: 'absolute',
    top: 0,
    left: 0,
  },
};

export default class ShopGrid extends React.Component {
  keyExtractor = item => `store_${item}`

  renderItem = ({ item, index }) => (
    <ShopCard
      compact
      peerID={item}
      key={`shop_${index}`}
      onPress={this.props.onPress}
      style={styles.card}
      contentWrapperStyle={styles.contentWrapper}
    />
  )

  render() {
    const { shops, count } = this.props;
    return (
      <View style={styles.mainWrapper}>
        <FlatList
          data={shops}
          renderItem={this.renderItem}
          keyExtractor={this.keyExtractor}
          numColumns={2}
          contentContainerStyle={styles.containerStyle}
          columnWrapperStyle={styles.columnWrapperStyle}
        />
        {(!shops || shops.length < count) && (
          <View style={styles.contentLoaderWrapper}>
            <ShopGridLoader />
          </View>
        )}
      </View>
    );
  }
}
