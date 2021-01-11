import React from 'react';
import { View, Text, Switch as NativeSwitch } from 'react-native';

import { formLabelColor, borderColor, primaryTextColor } from '../commonColors';

import Switch from './Switch';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    borderBottomWidth: 1,
    borderColor,
    paddingVertical: 14,
  },
  title: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
    paddingBottom: 8,
  },
  secondaryText: {
    color: primaryTextColor,
    flex: 1,
    width: 'auto',
  },
  input: {
    marginBottom: 8,
  },
};

export default class SwitchInput extends React.Component {
  render() {
    const {
      title, value, onChange, noBorder, secondary, useNative, style, wrapperStyle,
    } = this.props;
    return (
      <View style={[styles.wrapper, noBorder && { borderBottomWidth: 0 }, wrapperStyle]}>
        <Text style={[styles.title, secondary && styles.secondaryText, style]}>{title}</Text>
        {useNative ? (
          <NativeSwitch style={styles.input} value={value} onValueChange={onChange} />
        ) : (
          <Switch onChange={onChange} value={value} />
        )}
      </View>
    );
  }
}
