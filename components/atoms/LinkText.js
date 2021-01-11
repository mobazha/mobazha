import React from 'react';
import { View, Text } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  text: {
    fontSize: 16,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'right',
    color: brandColor,
    padding: 10,
  },
  arrowStyle: {
    paddingRight: 15,
  },
  arrowWrapper: {
    paddingRight: 20,
  },
  bold: {
    fontWeight: 'bold',
    textTransform: 'uppercase',
  },
  arrow: {
    marginTop: 2,
  },
};

export default ({
  style, text, color, fontSize, withArrow, bold,
}) => (
  <View style={[styles.wrapper, withArrow ? styles.arrowWrapper : {}]}>
    <Text
      style={[
        styles.text,
        fontSize ? { fontSize } : {},
        color ? { color } : {},
        withArrow ? styles.arrowStyle : {},
        bold ? styles.bold : {},
        style,
      ]}
    >
      {text}
    </Text>
    {withArrow && <Ionicons style={styles.arrow} size={16} name="md-send" color={color || brandColor} />}
  </View>
);
