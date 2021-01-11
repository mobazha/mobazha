import React from 'react';
import { TouchableWithoutFeedback, Text, View } from 'react-native';
import decode from 'unescape';

import { primaryTextColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingHorizontal: 11,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    marginRight: 6,
    marginBottom: 6,
  },
  text: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
};

export default ({ tag, onPress }) => (
  <TouchableWithoutFeedback onPress={() => onPress(tag)}>
    <View style={styles.wrapper}>
      <Text style={styles.text}>#{decode(tag)}</Text>
    </View>
  </TouchableWithoutFeedback>
);
