import React from 'react';
import { View, Text, TextInput, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { primaryTextColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
  },
  title: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    marginTop: 2,
    marginBottom: 4,
  },
  controlWrapper: {
    flexDirection: 'row',
    width: 92,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#e8e8e8',
  },
  button: {
    width: 30,
    height: 30,
    backgroundColor: '#f5f6f6',
    borderStyle: 'solid',
    justifyContent: 'center',
    alignItems: 'center',
  },
  input: {
    width: 30,
    height: 30,
    textAlign: 'center',
    borderRightWidth: 1,
    borderLeftWidth: 1,
    borderColor: '#e8e8e8',
    color: primaryTextColor,
    padding: 0,
  },
};

export default ({ value, onChange }) => (
  <View style={styles.wrapper}>
    <View style={styles.controlWrapper}>
      <TouchableWithoutFeedback
        onPress={() => {
          onChange(Math.max(1, value - 1));
        }}
      >
        <View style={styles.button}>
          <Ionicons name="ios-remove" size={24} color={primaryTextColor} />
        </View>
      </TouchableWithoutFeedback>
      <TextInput
        editable={false}
        style={styles.input}
        value={value.toString()}
        onChangeText={(val) => {
          onChange(val);
        }}
      />
      <TouchableWithoutFeedback
        onPress={() => {
          // onChange(Math.min(value + 1, maxValue));
          onChange(value + 1);
        }}
      >
        <View style={styles.button}>
          <Ionicons name="ios-add" size={24} color={primaryTextColor} />
        </View>
      </TouchableWithoutFeedback>
    </View>
  </View>
);
