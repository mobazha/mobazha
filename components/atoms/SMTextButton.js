import React, { Fragment } from 'react';
import { TouchableWithoutFeedback, Text, View, ActivityIndicator } from 'react-native';

import { brandColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'transparent',
    justifyContent: 'center',
    borderRadius: 4,
    flexDirection: 'row',
    alignItems: 'center',
  },
  text: {
    color: brandColor,
    fontSize: 15,
    lineHeight: 22,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
  },
  disabled: {
    opacity: 0.3,
  },
  loading: {
    marginLeft: 4,
  },
};

export default ({
  title, onPress, wrapperStyle, textStyle, disabled, loadingText,
}) => (
  <TouchableWithoutFeedback onPress={onPress} disabled={disabled || loadingText}>
    <View style={[styles.wrapper, wrapperStyle, disabled && styles.disabled]}>
      <Text style={[styles.text, textStyle]}>{loadingText || title}</Text>
      {loadingText && <ActivityIndicator style={styles.loading} size="small" color={brandColor} />}
    </View>
  </TouchableWithoutFeedback>
);
