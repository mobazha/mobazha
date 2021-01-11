import React from 'react';
import { Text, View } from 'react-native';

import Badge from './Badge';
import { formLabelColor, brandColor, bgHightlightColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  text: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
    paddingVertical: 20,
    minWidth: 150,
  },
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    height: 60,
  },
  coinsValue: {
    fontSize: 15,
    color: 'black',
  },
};

export default ({ text, value, style, isBeta }) => {
  if (!value) {
    return (
      <View style={styles.wrapper}>
        <Text style={[styles.text, style]}>{text}</Text>
        {isBeta && (
          <Badge text="BETA" color={brandColor} backgroundColor={bgHightlightColor} />
        )}
      </View>
    );
  }

  return (
    <View style={styles.wrapper}>
      <Text style={[styles.text, style]}>{text}</Text>
      <Text style={styles.coinsValue} numberOfLines={1}>{value}</Text>
    </View>
  );
};
