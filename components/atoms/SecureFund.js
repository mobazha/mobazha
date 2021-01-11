import React from 'react';
import { View, TouchableWithoutFeedback, Text } from 'react-native';

import { brandColor, primaryTextColor, bgHightlightColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: 16,
    backgroundColor: bgHightlightColor,
  },
  secureText: {
    fontSize: 15,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  button: {
    justifyContent: 'center',
    alignItems: 'center',
    height: 35,
    backgroundColor: '#fff',
    paddingHorizontal: 12,
    borderWidth: 1,
    borderRadius: 5,
    borderColor: brandColor,
  },
  btnText: {
    fontSize: 13,
    fontWeight: 'bold',
    letterSpacing: 0,
    textAlign: 'center',
    color: brandColor,
    lineHeight: 35,
    height: 35,
  },
};

export default ({ onBackup }) => (
  <View style={styles.wrapper}>
    <Text style={styles.secureText}>{I18n.t('components.atoms.SecureFund.secure_funds')}</Text>
    <TouchableWithoutFeedback onPress={onBackup}>
      <View style={styles.button}>
        <Text style={styles.btnText}>{I18n.t('components.atoms.SecureFund.backup_wallet')}</Text>
      </View>
    </TouchableWithoutFeedback>
  </View>
);
