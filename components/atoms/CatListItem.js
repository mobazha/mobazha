import React from 'react';
import { TouchableWithoutFeedback, Image, Text, View } from 'react-native';

import { primaryTextColor, bgHightlightColor, borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    height: 72,
    alignItems: 'center',
  },
  text: {
    fontSize: 16,
    fontWeight: 'bold',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  image: {
    width: 44,
    height: 44,
    marginLeft: 16,
    marginRight: 12,
    backgroundColor: borderColor,
    borderRadius: 22,
  },
};

export default ({ item, onPress }) => (
  <TouchableWithoutFeedback onPress={onPress} activeOpacity={1}>
    <View style={styles.wrapper}>
      <Image source={item.image} style={styles.image} />
      <Text style={styles.text}>{item.title}</Text>
    </View>
  </TouchableWithoutFeedback>
);
