import React from 'react';
import { View, TouchableWithoutFeedback, Text } from 'react-native';
import Feather from 'react-native-vector-icons/Feather';

import { primaryTextColor, borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingHorizontal: 19,
    paddingTop: 7,
    paddingBottom: 5,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    borderTopWidth: 0,
    backgroundColor: '#FFF',
    borderColor,
    borderBottomWidth: 1,
  },
  noBorder: {
    borderBottomWidth: 0,
  },
  modeIconWrapper: {},
  counts: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
};

export default ({
  mode, onChange, counts, noBorder,
}) => (
  <View style={[styles.wrapper, noBorder ? styles.noBorder : {}]}>
    <Text style={styles.counts}>{counts !== undefined ? I18n.t('components.organism.ProductModeSelector.listings', {counts: counts}) : ''}</Text>
    <TouchableWithoutFeedback
      onPress={() => {
        onChange(mode === 'card' ? 'list' : 'card');
      }}
    >
      <View style={styles.modeIconWrapper}>
        {mode === 'card' ? (
          <Feather name="grid" size={25} />
        ) : (
          <Feather name="menu" size={25} />
        )}
      </View>
    </TouchableWithoutFeedback>
  </View>
);
