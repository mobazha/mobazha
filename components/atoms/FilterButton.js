import React from 'react';
import { TouchableWithoutFeedback, View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { primaryTextColor, borderColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const wrapperStyle = {
  paddingHorizontal: 10,
};

export default ({ disabled, onPress, secondary }) => (
  <TouchableWithoutFeedback onPress={onPress} disabled={disabled}>
    <View style={wrapperStyle}>
      <Ionicons
        name="md-funnel"
        color={disabled ? borderColor : secondary ? foregroundColor : primaryTextColor}
        size={24}
      />
    </View>
  </TouchableWithoutFeedback>
);
