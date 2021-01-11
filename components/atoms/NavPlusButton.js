import React from 'react';
import { View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    width: 32,
    height: 32,
    justifyContent: 'center',
  },
};

const NavPlusButton = () => (
  <View style={styles.container}>
    <Ionicons
      name="md-add"
      size={24}
      style={{ textAlign: 'center', color: 'white' }}
    />
  </View>
);

export default NavPlusButton;
