import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import Icons from 'react-native-vector-icons/MaterialIcons';
import InputGroup from '../atoms/InputGroup';
import { primaryTextColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    paddingVertical: 12,
  },
  radioButton: {
    width: 20,
    height: 20,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 10,
  },
  raidoButtonFill: {
    width: 16,
    height: 16,
    borderRadius: 8,
    backgroundColor: foregroundColor,
  },
  active: {
    backgroundColor: primaryTextColor,
  },
  option: {
    flex: 1,
  },
  optionTitle: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    lineHeight: 19,
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  optionDescription: {
    width: '100%',
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 19,
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
};

export default ({ selected, onPress }) => (
  <InputGroup title="Direct Payment">
    <TouchableWithoutFeedback onPress={onPress}>
      <View style={styles.wrapper}>
        <View style={[styles.radioButton]}>
          <View style={[styles.raidoButtonFill, selected ? styles.active : {}]} />
        </View>
        <View style={styles.option}>
          <Text style={styles.optionTitle}>{I18n.t('components.molecules.DirectPaymentOption.direct_payment')}</Text>
          <Text style={styles.optionDescription}>{I18n.t('components.molecules.DirectPaymentOption.description')}</Text>
        </View>
      </View>
    </TouchableWithoutFeedback>
  </InputGroup>
);
