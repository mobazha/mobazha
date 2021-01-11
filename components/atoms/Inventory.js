import React, { PureComponent } from 'react';
import { View, Text } from 'react-native';
import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'column',
    paddingVertical: 5,
  },
  sku: {
    fontSize: 12,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  surcharge: {
    fontSize: 12,
    color: primaryTextColor,
  },
  stock: {
    fontSize: 12,
    color: primaryTextColor,
  },
};

export default class Inventory extends PureComponent {
  render() {
    const { inventory } = this.props;
    return (
      <View style={styles.wrapper}>
        <Text style={styles.sku}>
          {inventory.productId}
        </Text>
        <Text style={styles.surcharge}>
          {`${I18n.t('components.atomsInventory.surcharge')}: $${inventory.surcharge}`}
        </Text>
        <Text style={styles.stock}>
          {`${I18n.t('components.atomsInventory.stock')}: ${inventory.quantity === -1 ? I18n.t('components.atomsInventory.unlimited') : inventory.quantity}`}
        </Text>
      </View>
    );
  }
}
