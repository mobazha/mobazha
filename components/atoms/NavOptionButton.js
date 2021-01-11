import React from 'react';
import { View } from 'react-native';
import Icons from 'react-native-vector-icons/MaterialIcons';

import { primaryTextColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    width: 24,
    height: 32,
    justifyContent: 'center',
  },
};

const NavOptionButton = ({ white }) => (
  <View style={styles.container}>
    <Icons
      name="more-vert"
      size={24}
      color={white ? foregroundColor : primaryTextColor}
    />
  </View>
)

export default NavOptionButton;
