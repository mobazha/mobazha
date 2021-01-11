import React from 'react';
import { View, Text, Image } from 'react-native';

import HollowButton from '../atoms/HollowButton';

import CouponIcon from '../../assets/icons/shipping.png';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
  },
  text: {
    fontSize: 14,
    color: '#8a8a8f',
    marginBottom: 10,
  },
  img: {
    width: 67.4,
    height: 49,
    marginBottom: 11.5,
  },
};

export default ({ onAdd }) => (
  <View style={styles.wrapper}>
    <Image style={styles.img} source={CouponIcon} />
    <Text style={styles.text}>{I18n.t('components.organism.EmptyShippingMethods.empty_shipping_option')}</Text>
    <HollowButton title={I18n.t('components.organism.EmptyShippingMethods.add_shipping')} onPress={onAdd} />
  </View>
);
