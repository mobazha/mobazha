import React from 'react';
import { View, Text, Platform } from 'react-native';

import { navHeightStyle } from '../../utils/navbar';

import {I18n} from '../../langs/I18n';

const styles = {
  modalClose: {
    position: 'absolute',
    height: 32,
    marginTop: (navHeightStyle.height - 32) / 2,
    marginLeft: 40,
    paddingLeft: (Platform.OS === 'ios' ? 6 : 16),
    justifyContent: 'center',
    zIndex: 999,
  },
  text: {
    color: '#FFFFFF',
    fontSize: 15,
  },
};

const ModalImageIndicator = ({ pos, size }) => (
  <View style={styles.modalClose}>
    <Text style={styles.text}>
      {I18n.t('components.atoms.ModalImageIndicator.posInfo', { pos: pos + 1, size: size })}
    </Text>
  </View>
);

export default ModalImageIndicator;
