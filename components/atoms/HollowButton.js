import React from 'react';
import { TouchableWithoutFeedback, Text, View } from 'react-native';
import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    minWidth: 250,
    height: 29,
    borderRadius: 2,
    backgroundColor: '#ffffff',
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    alignItems: 'center',
    justifyContent: 'center',
  },
  title: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  small: {
    minWidth: 83,
  },
};

export default ({
  style, title, onPress, small,
}) => (
  <TouchableWithoutFeedback
    onPress={onPress}
  >
    <View style={[styles.wrapper, style, (small && styles.small) || []]}>
      <Text style={styles.title}>{title}</Text>
    </View>
  </TouchableWithoutFeedback>
);
