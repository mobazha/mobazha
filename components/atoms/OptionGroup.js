import React from 'react';
import { View, TouchableWithoutFeedback } from 'react-native';
import Entypo from 'react-native-vector-icons/Entypo';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { primaryTextColor, borderColor, formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingVeritical: 0,
    borderBottomWidth: 1,
    borderColor,
    justifyContent: 'space-between',
    alignItems: 'center',
    flexDirection: 'row',
    alignSelf: 'stretch',
    paddingRight: 16,
  },
  noBorder: { borderBottomWidth: 0 },
  smallPadding: { paddingRight: 0 },
  content: { width: '70%' },
  icon: { paddingRight: 0 },
};

export default ({
  onPress, children, noBorder, smallPadding, noArrow, contentStyle, style, fullWidth, onClear,
}) => (
  <TouchableWithoutFeedback disabled={!onPress} onPress={onPress}>
    <View
      style={[
        styles.wrapper,
        noBorder ? styles.noBorder : {},
        smallPadding ? styles.smallPadding : {},
        style,
      ]}
    >
      <View style={[styles.content, noArrow ? { width: '100%' } : {}, fullWidth ? { flex: 1 } : {}, contentStyle]}>{children}</View>
      {!noArrow && (
        <Entypo name="chevron-thin-right" size={18} color={primaryTextColor} style={styles.icon} />
      )}
      {onClear && (
        <TouchableWithoutFeedback onPress={onClear}>
          <Ionicons name="md-close-circle" size={24} color={formLabelColor} style={styles.icon} />
        </TouchableWithoutFeedback>
      )}
    </View>
  </TouchableWithoutFeedback>
);
