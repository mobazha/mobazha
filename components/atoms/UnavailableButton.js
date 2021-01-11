import React from 'react';
import { TouchableOpacity, Text, Platform, View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { brandColor, primaryTextColor, foregroundColor, mainBorderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    marginVertical: 15,
    marginHorizontal: 12,
    paddingHorizontal: 17,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: mainBorderColor,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
  },
  btnText: {
    color: primaryTextColor,
    fontWeight: 'bold',
  },
  btnIcon: {
    marginLeft: 3,
    marginTop: 2,
    width: 16,
    height: 16,
    alignItems: 'center',
    justifyContent: 'center',
  },
};

export default ({ onPress }) => (
  <TouchableOpacity style={styles.wrapper} onPress={onPress}>
    <Text style={styles.btnText}>{I18n.t('components.atoms.UnavailableButton.unavailable')}</Text>
    <View style={styles.btnIcon}>
      <Ionicons
        name={Platform.OS === 'ios' ? 'ios-information-circle-outline' : 'md-information-circle-outline'}
        color={brandColor}
        size={14}
      />
    </View>
  </TouchableOpacity>
);
