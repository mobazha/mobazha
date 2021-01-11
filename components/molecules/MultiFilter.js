import React, { PureComponent } from 'react';
import { TouchableWithoutFeedback, Text, View, ScrollView } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { findIndex, remove, get } from 'lodash';

import { foregroundColor, greenColor, primaryTextColor, borderColor } from '../commonColors';

import Section from './Section';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor,
  },
  itemWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 14,
  },
  checkmark: {
    color: greenColor,
    marginRight: 14,
    width: 14,
  },
  itemTextWrapper: {
    flex: 1,
    paddingVertical: 20,
  },
  bottomBorder: {
    borderBottomWidth: 1,
    borderColor,
  },
  itemText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
};

export default class MultiFilter extends PureComponent {
  onChange(val, idx, isExclusive) {
    const { selected, options } = this.props;
    const exclusiveIdx = findIndex(options, val => get(val, 'exclusive', false));
    const exclusiveVal = get(options, `[${exclusiveIdx}].value`, 'exclusive_value');
    if (idx === -1) {
      if (isExclusive) {
        selected.length = 0;
        selected.push(val);
      } else {
        selected.push(val);
        remove(selected, o => o === exclusiveVal);
      }
    } else {
      remove(selected, o => o === val);
    }
    this.props.onChange(selected);
  }

  render() {
    const {
      selected, options, title, style, hasScroll,
    } = this.props;
    const Wrapper = hasScroll ? ScrollView : View;
    return (
      <Section title={title}>
        <Wrapper style={[styles.wrapper, style]}>
          {options.map((val, key) => {
            const idx = findIndex(selected, o => o === val.value);
            const isLast = key === options.length - 1;
            return (
              <TouchableWithoutFeedback
                key={key}
                onPress={() => {
                  this.onChange(val.value, idx, get(val, 'exclusive', false));
                }}
              >
                <View style={styles.itemWrapper}>
                  {idx >= 0 ? (
                    <Ionicons name="ios-checkmark" size={28} style={styles.checkmark} />
                  ) : (
                    <Ionicons size={28} style={styles.checkmark} />
                  )}
                  <View style={[styles.itemTextWrapper, !isLast && styles.bottomBorder]}>
                    <Text style={styles.itemText}>{val.label}</Text>
                  </View>
                </View>
              </TouchableWithoutFeedback>
            );
          })}
        </Wrapper>
      </Section>
    );
  }
}
