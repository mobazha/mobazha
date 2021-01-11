import React from 'react';
import { View, Text, Image } from 'react-native';

import HollowButton from '../atoms/HollowButton';

import CouponIcon from '../../assets/icons/coupon.png';

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
    width: 49.1,
    height: 49.1,
    marginBottom: 11.5,
  },
};

export default ({ onAdd }) => (
  <View style={styles.wrapper}>
    <Image style={styles.img} source={CouponIcon} />
    <Text style={styles.text}>{I18n.t('components.organism.EmptyCoupons.empty_coupon')}</Text>
    <HollowButton title={I18n.t('components.organism.EmptyCoupons.add_coupon')} onPress={onAdd} />
  </View>
);
