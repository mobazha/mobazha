import React, { PureComponent } from 'react';
import { View, Text, TouchableOpacity, FlatList } from 'react-native';
import { get } from 'lodash';
import { connect } from 'react-redux';

import { convertorsMap } from '../../selectors/currency';
import { primaryTextColor, secondaryTextColor } from '../commonColors';
import OptionButton from '../atoms/NavOptionButton';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingHorizontal: 16,
    paddingVertical: 20,
    width: '100%',
    borderBottomWidth: 1,
    borderColor: '#e8e8e8',
  },
  titleWrapper: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 12,
    width: '100%',
  },
  title: {
    fontSize: 16,
    fontWeight: 'bold',
    color: primaryTextColor,
    width: 220,
  },
  serviceWrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    marginBottom: 4,
  },
  fieldWrapper: {
    width: 220,
    flexDirection: 'column',
  },
  serviceName: {
    fontSize: 15,
    color: primaryTextColor,
    marginRight: 10,
  },
  estDelivery: {
    fontSize: 15,
    color: secondaryTextColor,
  },
  price: {
    fontSize: 14,
    color: '#00BF65',
  },
};

class ShippingMethod extends PureComponent {
  onPress = () => {
    const { pos } = this.props;
    this.props.onClickOption(pos);
  };

  keyExtractor = ({ index }) => `service_${index}`;

  renderPrice = (price) => {
    const { localLabelFromLocal } = this.props;
    return parseFloat(price) === 0 ? (
      <Text style={styles.price}>{I18n.t('components.organism.ShippingMethod.free')}</Text>
    ) : (
      <Text style={styles.price}>{localLabelFromLocal(parseFloat(price))}</Text>
    );
  };

  renderService = ({ item }) => (
    <View style={styles.serviceWrapper}>
      <View style={styles.fieldWrapper}>
        <Text style={styles.serviceName}>{get(item, 'name', '')}</Text>
        <Text style={styles.estDelivery}>{get(item, 'estimatedDelivery', '')}</Text>
      </View>
      {this.renderPrice(get(item, 'price', 0))}
    </View>
  );

  render() {
    const { name, services } = this.props.method;
    return (
      <View style={styles.wrapper}>
        <View style={styles.titleWrapper}>
          <Text style={styles.title}>{name}</Text>
          <TouchableOpacity onPress={this.onPress}>
            <OptionButton />
          </TouchableOpacity>
        </View>
        <FlatList
          keyExtractor={this.keyExtractor}
          data={services}
          renderItem={this.renderService}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(ShippingMethod);
