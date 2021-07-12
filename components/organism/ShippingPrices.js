import React, { PureComponent } from 'react';
import { View, FlatList, Alert, KeyboardAvoidingView } from 'react-native';

import MoreButton from '../atoms/MoreButton';
import ShippingPriceEditor from './ShippingPriceEditor';
import { keyboardAvoidingViewSharedProps } from '../../utils/keyboard';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    flexDirection: 'column',
  },
  moreButtonWrapper: {
    paddingVertical: 19,
    paddingHorizontal: 15,
  },
};

export default class ShippingPrices extends PureComponent {
  constructor(props) {
    super(props);
    const services = [...props.services];
    if (services.length === 0) {
      services.push({
        name: '',
        price: '',
        estimatedDelivery: '',
        additionalItemPrice: '0',
      });
    }
    this.state = {
      services,
    };
  }

  onChangeItem = (item, pos) => {
    const services = [...this.state.services];
    services[pos] = item;
    this.setState({ services });
    this.props.onChange(services);
  };

  onRemoveItem = pos => () => {
    Alert.alert(I18n.t('components.organism.ShippingPrices.delete_service'), I18n.t('components.organism.ShippingPrices.cannot_undo'), [
      { text: I18n.t('components.organism.ShippingPrices.cancel') },
      {
        text: I18n.t('components.organism.ShippingPrices.delete'),
        onPress: () => this.removeItem(pos),
      },
    ]);
  }

  removeItem = (pos) => {
    const services = [...this.state.services];
    services.splice(pos, 1);
    this.setState({ services });
    this.props.onChange(services);
  }

  newItem = () => {
    const services = [...this.state.services];
    services.push({
      name: '',
      price: '0',
      estimatedDelivery: '',
      additionalItemPrice: '0',
    });
    this.setState({ services });
    this.props.onChange(services);
  };

  keyExtractor = (item, index) => `shipping_price_${index}`;

  renderItem = ({ item, index }) => {
    const { services } = this.state;
    return (
      <ShippingPriceEditor
        item={item}
        pos={index}
        onChange={this.onChangeItem}
        removeItem={this.onRemoveItem(index)}
        hideDelete={services.length <= 1}
      />
    );
  };

  render() {
    const { services } = this.state;
    return (
      <KeyboardAvoidingView style={styles.wrapper} {...keyboardAvoidingViewSharedProps}>
        <FlatList
          data={services}
          renderItem={this.renderItem}
          keyExtractor={this.keyExtractor}
          ListFooterComponent={
            <View style={styles.moreButtonWrapper}>
              <MoreButton title={I18n.t('components.organism.ShippingPrices.add_service')} onPress={this.newItem} />
            </View>
          }
          extraData={services.length}
        />
      </KeyboardAvoidingView>
    );
  }
}
