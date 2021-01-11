import React from 'react';
import { Text } from 'react-native';

import { primaryTextColor } from '../../components/commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  header: {
    fontSize: 16,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: -0.1,
    color: primaryTextColor,
    paddingTop: 20,
    paddingLeft: 15,
    paddingBottom: 7,
  },
};

export const renderTextHeader = text => ({ style }) => (
  <Text style={[styles.header, style]}>{text}</Text>
);
