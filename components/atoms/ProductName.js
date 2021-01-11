import React from 'react';
import { View, Text } from 'react-native';
import decode from 'unescape';

import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    marginTop: 15,
    paddingHorizontal: 20,
  },
  text: {
    fontSize: 17,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
};

export default ({ prodName }) => (
  <View style={styles.wrapper}>
    <Text style={styles.text}>{decode(prodName)}</Text>
  </View>
);
