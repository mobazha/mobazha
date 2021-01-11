import React, { PureComponent } from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import Entypo from 'react-native-vector-icons/Entypo';
import { findIndex } from 'lodash';

import { formLabelColor, primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    height: 60,
  },
  borderBottom: {
    borderBottomWidth: 1,
    borderColor: '#e8e8e8',
  },
  title: {
    width: 150,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  input: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    paddingVertical: 20,
    paddingRight: 15,
  },
  optionTrigger: {
    flex: 1,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  option: {
    paddingVertical: 20,
    borderBottomWidth: 1,
    borderColor: '#c8c7cc',
    flex: 1,
  },
  selected: {
    backgroundColor: '#c8c7cc',
  },
  optionText: {
    color: primaryTextColor,
    fontSize: 16,
  },
  iconForward: {
    paddingRight: 0,
  },
  checkMark: {
    width: 18,
    marginRight: 10,
    alignItems: 'center',
    justifyContent: 'center',
  },
  optionWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
};

export default class SelectorPlaceholder extends PureComponent {
  getOptionLabel(val) {
    const { options } = this.props;
    const idx = findIndex(options, o => o.value === val);
    return idx >= 0 ? options[idx].label : 'Select';
  }
  render() {
    const {
      title, noMargin, noBorder, required, onPress, value,
    } = this.props;
    let text = 'Select';
    if (value) {
      text = value.label ? value.label : this.getOptionLabel(value);
    }
    return (
      <View
        style={[
          styles.wrapper,
          noBorder ? {} : styles.borderBottom,
          noMargin ? { marginHorizontal: 0 } : {},
        ]}
      >
        <Text style={styles.title}>
          {title}
          <Text style={required ? { color: 'red' } : {}}>{required ? '*' : ''}</Text>
        </Text>
        <TouchableWithoutFeedback onPress={onPress}>
          <View style={styles.optionTrigger}>
            <Text style={styles.input}>{text}</Text>
            <Entypo
              name="chevron-thin-right"
              size={18}
              color={primaryTextColor}
              style={styles.iconForward}
            />
          </View>
        </TouchableWithoutFeedback>
      </View>
    );
  }
}
