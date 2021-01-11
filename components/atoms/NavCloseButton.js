import React from 'react';
import { Platform, View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    width: 32,
    height: 32,
    justifyContent: 'center',
  },
  iosStyle: {
    paddingHorizontal: 10,
  },
  androidStyle: {
    textAlign: 'center',
  },
};

const NavCloseButton = () =>
  (Platform.OS === 'ios' ? (
    <Ionicons name="ios-close" size={32} color={primaryTextColor} style={styles.iosStyle} />
  ) : (
    <View style={styles.container}>
      <Ionicons name="md-close" size={24} color={primaryTextColor} style={styles.androidStyle} />
    </View>
  ));

export default NavCloseButton;
