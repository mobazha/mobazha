import React from 'react';
import { View, Text } from 'react-native';
import { backgroundColor, primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  section: {
    marginTop: 20,
    backgroundColor,
  },
  sectionBody: {
    marginTop: 7,
  },
  title: {
    fontSize: 16,
    fontWeight: 'bold',
    fontStyle: 'normal',
    textAlign: 'left',
    letterSpacing: -0.1,
    color: primaryTextColor,
    paddingLeft: 15,
  },
};

export default ({
  style, title, children, bodyStyle,
}) => (
  <View style={[styles.section, style]}>
    <Text style={styles.title}>{title}</Text>
    <View style={[styles.sectionBody, bodyStyle]}>{children}</View>
  </View>
);
