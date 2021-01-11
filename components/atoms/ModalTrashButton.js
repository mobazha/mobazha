import React from 'react';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { TouchableWithoutFeedback, Platform, View } from 'react-native';

import { navHeightStyle } from '../../utils/navbar';

import {I18n} from '../../langs/I18n';

const styles = {
  modalTrash: {
    position: 'absolute',
    width: 32,
    height: 32,
    right: 20,
    marginTop: (navHeightStyle.height - 32) / 2,
    paddingLeft: (Platform.OS === 'ios' ? 6 : 16),
    justifyContent: 'center',
    zIndex: 999,
  },
  icon: {
    justifyContent: 'flex-start',
  },
};

const ModalTrashButton = ({ onPress }) => (
  <TouchableWithoutFeedback
    onPress={onPress}
  >
    <View style={styles.modalTrash}>
      <Ionicons
        style={styles.icon}
        name={Platform.OS === 'ios' ? 'ios-trash' : 'md-trash'}
        size={Platform.OS === 'ios' ? 28 : 24}
        color="white"
      />
    </View>
  </TouchableWithoutFeedback>
);

export default ModalTrashButton;
