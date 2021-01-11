import React from 'react';
import { TouchableWithoutFeedback, View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { borderColor, formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  button: {
    width: 46,
    height: 33,
    borderRadius: 2,
    borderWidth: 1,
    borderColor,
    alignItems: 'center',
    justifyContent: 'center',
  },
};

export default ({ onPress }) => (
  <TouchableWithoutFeedback onPress={onPress}>
    <View style={styles.button}>
      <Ionicons name="md-camera" size={16} color={formLabelColor} />
    </View>
  </TouchableWithoutFeedback>
);
