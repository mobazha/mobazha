import React from 'react';
import { View, TouchableWithoutFeedback, Text } from 'react-native';

import { borderColor, primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    height: 60,
    borderTopWidth: 1,
    borderColor,
  },
  text: {
    fontSize: 15,
    color: primaryTextColor,
    fontWeight: 'bold',
  },
};

export default ({ onPress }) => (
  <TouchableWithoutFeedback onPress={onPress}>
    <View style={styles.wrapper}><Text style={styles.text}>{I18n.t('components.atoms.ResetFilter.reset_filters')}</Text></View>
  </TouchableWithoutFeedback>
);
