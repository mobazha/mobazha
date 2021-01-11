import React from 'react';
import { Platform, View, Animated } from 'react-native';
import EvilIcons from 'react-native-vector-icons/EvilIcons';

import { foregroundColor, primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    width: 32,
    height: 32,
    justifyContent: 'center',
  },
  iosStyle: {
    textAlign: 'center',
    marginTop: 5,
  },
  androidStyle: {
    textAlign: 'center',
  },
};

const NavShareButton = ({ white, animColor }) => {
  const IconComponent = animColor ? Animated.createAnimatedComponent(EvilIcons) : EvilIcons;
  const color = animColor || (white ? foregroundColor : primaryTextColor);

  return (Platform.OS === 'ios' ? (
    <View style={[styles.container, styles.iosStyle]}>
      <IconComponent
        name="share-apple"
        size={32}
        color={!animColor && color}
        style={animColor && { color: animColor }}
      />
    </View>
  ) : (
    <View style={styles.container}>
      <IconComponent
        name="share-google"
        size={24}
        color={!animColor && color}
        style={[styles.androidStyle, animColor && { color: animColor }]}
      />
    </View>
  ));
};

export default NavShareButton;
