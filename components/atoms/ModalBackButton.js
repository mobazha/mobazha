import React from 'react';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { TouchableWithoutFeedback, Platform, View } from 'react-native';

import { navHeightStyle } from '../../utils/navbar';

import {I18n} from '../../langs/I18n';

const styles = {
  modalBack: {
    position: 'absolute',
    width: 32,
    height: 32,
    marginTop: (navHeightStyle.height - 32) / 2,
    marginLeft: 10,
    paddingLeft: (Platform.OS === 'ios' ? 6 : 16),
    justifyContent: 'center',
    zIndex: 999,
  },
  icon: {
    justifyContent: 'flex-start',
  },
};

const ModalBackButton = ({ onPress }) => (
  <TouchableWithoutFeedback
    onPress={onPress}
  >
    <View style={styles.modalBack}>
      <Ionicons
        style={styles.icon}
        name={Platform.OS === 'ios' ? 'ios-arrow-back' : 'md-arrow-back'}
        size={Platform.OS === 'ios' ? 32 : 24}
        color="#eee"
      />
    </View>
  </TouchableWithoutFeedback>
);

export default ModalBackButton;
