import React from 'react';
import { TouchableWithoutFeedback, Image, Text, View, Dimensions } from 'react-native';

import { secondaryTextColor, borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const { width: screenWidth } = Dimensions.get('screen');

const styles = {
  wrapper: {
    width: screenWidth / 5,
    height: 65,
    alignItems: 'center',
    justifyContent: 'center',
    paddingTop: 0,
  },
  text: {
    fontSize: 12,
    color: secondaryTextColor,
  },
  image: {
    width: 44,
    height: 44,
    marginBottom: 5,
    backgroundColor: borderColor,
    borderRadius: 22,
  },
};

export default ({ item, onPress }) => (
  <TouchableWithoutFeedback onPress={onPress} activeOpacity={1}>
    <View style={styles.wrapper}>
      <Image source={item.image} style={styles.image} />
      <Text numberOfLines={1} style={styles.text}>{item.short || item.name}</Text>
    </View>
  </TouchableWithoutFeedback>
);
