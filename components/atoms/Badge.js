import React from 'react';
import { View, Text } from 'react-native';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingHorizontal: 6,
    height: 20,
    borderRadius: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  text: {
    fontSize: 11,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
  },
};

export default ({
  style, backgroundColor = '#00BF65', color = '#FFFFFF', text,
}) => (
  <View style={[styles.wrapper, { backgroundColor }, style || {}]}>
    <Text style={[styles.text, { color }]}>{text}</Text>
  </View>
);
