import React from 'react';
import { FlatList, Text } from 'react-native';

import InventoryItem from '../organism/InventoryItem';

import {I18n} from '../../langs/I18n';

const styles = {
  text: {
    fontSize: 14,
    color: '#8d8d8d',
    textAlign: 'left',
    marginLeft: 16,
    marginBottom: 12,
  },
};

export default class InventoryList extends React.Component {
  keyExtractor = (item, index) => `inventory_${index}`;

  renderHeader = () => {
    const { inventory } = this.props;
    const count = inventory.length;
    return (
      <Text style={styles.text}>
        {I18n.t('components.templates.InventoryList.combos_info')}         
      </Text>
    );
  }

  renderItem = ({ item, index }) => {
    const { inventory, toItem, isTracking } = this.props;
    return (
      <InventoryItem
        item={item}
        isTracking={isTracking}
        isLast={index === inventory.length - 1}
        onPress={() => toItem(index)}
      />
    );
  };

  render() {
    const { inventory } = this.props;
    return (
      <FlatList
        data={inventory}
        renderItem={this.renderItem}
        ListHeaderComponent={this.renderHeader}
        keyExtractor={this.keyExtractor}
      />
    );
  }
}
