import React from 'react';
import { TouchableWithoutFeedback, Text, View, ActivityIndicator } from 'react-native';

import { brandColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: brandColor,
    margin: 15,
    paddingVertical: 8,
    justifyContent: 'center',
    borderRadius: 4,
    width: 150,
  },
  text: {
    color: foregroundColor,
    fontSize: 14,
    lineHeight: 22,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
  },
  disabled: {
    opacity: 0.3,
  },
};

export default ({
  title, onPress, wrapperStyle, textStyle, disabled, loading,
}) => (
  <TouchableWithoutFeedback onPress={onPress} disabled={disabled || loading}>
    <View style={[styles.wrapper, wrapperStyle, disabled && styles.disabled]}>
      {loading ? (
        <ActivityIndicator size="small" color="white" />
      ) : (
        <Text style={[styles.text, textStyle]}>{title}</Text>
      )}
    </View>
  </TouchableWithoutFeedback>
);
