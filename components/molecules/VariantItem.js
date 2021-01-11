import React from 'react';
import { View, Text } from 'react-native';
import { borderColor, primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    borderBottomWidth: 1,
    borderBottomColor: borderColor,
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 15,
  },
  noBorder: {
    borderBottomColor: 'transparent',
  },
  variantName: {
    fontSize: 15,
    color: primaryTextColor,
    width: 100,
  },
  variantOptionsHolder: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
  },
  variantOption: {
    paddingVertical: 7,
    paddingHorizontal: 11,
    fontSize: 13,
    color: primaryTextColor,
    fontWeight: 'bold',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    marginLeft: 6,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 5,
  },
};

export default ({ item: { name, variants }, isLast }) => (
  <View style={[styles.wrapper, (isLast && styles.noBorder) || {}]}>
    <Text style={styles.variantName}>{name}</Text>
    <View style={styles.variantOptionsHolder}>
      {variants.map((val, idx) => (
        <Text style={styles.variantOption} key={`variant_option_${idx}`}>
          {val}
        </Text>
      ))}
    </View>
  </View>
);
