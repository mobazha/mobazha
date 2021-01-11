import React from 'react';
import { TouchableWithoutFeedback, ActivityIndicator, View, Text } from 'react-native';
import { brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  button: {
    width: 118,
    height: 33,
    borderRadius: 2,
    backgroundColor: brandColor,
    alignItems: 'center',
    justifyContent: 'center',
  },
  text: {
    fontSize: 14,
    color: 'white',
    textAlign: 'center',
    fontWeight: 'bold',
  },
  disabled: {
    opacity: 0.35,
  },
};

export default ({ disabled, loading, onPress }) => (
  <TouchableWithoutFeedback disabled={disabled || loading} onPress={onPress}>
    <View style={[styles.button, disabled ? styles.disabled : {}]}>
      {loading ? (
        <ActivityIndicator size="small" color="white" />
      ) : (
        <Text style={styles.text}>{I18n.t('components.atoms.PostButton.post')}</Text>
      )}
    </View>
  </TouchableWithoutFeedback>
);
