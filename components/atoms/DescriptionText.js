import React from 'react';
import { Text } from 'react-native';

import {I18n} from '../../langs/I18n';

const textStyle = {
  marginVertical: 16,
  fontSize: 15,
  color: '#404040',
  lineHeight: 26,
};

export default ({ children, style }) => (
  <Text style={[textStyle, style]}>
    {children}
  </Text>
);
