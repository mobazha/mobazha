import React from 'react';
import { View, Text, TouchableOpacity } from 'react-native';
import { isEmpty } from 'lodash';

import { primaryTextColor, foregroundColor, borderColor } from '../commonColors';
import { navHeightStyle } from '../../utils/navbar';
import StatusBarWrapper from '../../status-bar';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    alignSelf: 'stretch',
  },
  contentWrapper: {
    paddingLeft: 6,
    paddingRight: 6,
    ...navHeightStyle,
    backgroundColor: 'white',
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    borderBottomWidth: 0,
    borderColor,
  },
  left: {
    width: 80,
    alignItems: 'flex-start',
  },
  center: {
    flex: 1,
    justifyContent: 'center',
  },
  right: {
    width: 80,
    alignItems: 'flex-end',
  },
  title: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  fillup: {
    flex: 1,
  },
};

export default ({
  title, left, right, rightDisabled, onLeft, onRight, hasBorder, modal,
}) => (
  <View style={styles.wrapper}>
    {!modal && <StatusBarWrapper backgroundColor={foregroundColor} barStyle="dark-content" />}
    <View style={[styles.contentWrapper, hasBorder && { borderBottomWidth: 1 }]}>
      <View style={styles.left}>
        <TouchableOpacity onPress={onLeft}>
          {left}
        </TouchableOpacity>
      </View>
      <View style={styles.center}>
        <Text style={styles.title}>{title}</Text>
      </View>
      <View style={[styles.right, isEmpty(title) ? styles.fillup : {}]}>
        <TouchableOpacity onPress={onRight} disabled={rightDisabled}>{right}</TouchableOpacity>
      </View>
    </View>
  </View>
);
