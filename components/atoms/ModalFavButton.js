import React from 'react';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { TouchableWithoutFeedback, Platform, View } from 'react-native';

import { navHeightStyle } from '../../utils/navbar';

import {I18n} from '../../langs/I18n';

const styles = {
  modalFav: {
    position: 'absolute',
    width: 32,
    height: 32,
    marginTop: (navHeightStyle.height - 32) / 2,
    right: 62,
    paddingLeft: (Platform.OS === 'ios' ? 6 : 10),
    justifyContent: 'center',
    zIndex: 999,
  },
  icon: {
    justifyContent: 'flex-start',
  },
};

const ModalFavButton = ({ selected, onPress }) => {
  let iconName = 'ios-star';
  if (Platform.OS === 'ios') {
    iconName = selected ? 'ios-star' : 'ios-star-outline';
  } else {
    iconName = selected ? 'md-star' : 'md-star-outline';
  }
  return (
    <TouchableWithoutFeedback
      onPress={onPress}
    >
      <View style={styles.modalFav}>
        <Ionicons
          style={styles.icon}
          name={iconName}
          size={Platform.OS === 'ios' ? 28 : 24}
          color={selected ? '#00bf65' : 'white'}
        />
      </View>
    </TouchableWithoutFeedback>
  );
};

export default ModalFavButton;
