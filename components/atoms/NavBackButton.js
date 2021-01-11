import React from 'react';
import { Platform, View, Animated } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { foregroundColor, primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    width: 32,
    height: 32,
    justifyContent: 'center',
  },
  iosStyle: {
    paddingLeft: 9,
  },
  androidStyle: {
    textAlign: 'center',
  },
};

const NavBackButton = ({ white, animColor }) => {
  const IconComponent = animColor ? Animated.createAnimatedComponent(Ionicons) : Ionicons;
  const color = animColor || (white ? foregroundColor : primaryTextColor);

  return (Platform.OS === 'ios' ? (
    <View style={[styles.container, styles.iosStyle]}>
      <IconComponent
        name="ios-arrow-back"
        size={32}
        color={!animColor && color}
        style={animColor && { color: animColor }}
      />
    </View>
  ) : (
    <View style={styles.container}>
      <IconComponent
        name="md-arrow-back"
        size={24}
        color={!animColor && color}
        style={[styles.androidStyle, animColor && { color: animColor }]}
      />
    </View>
  ));
};

export default NavBackButton;
