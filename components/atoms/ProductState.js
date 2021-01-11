import React from 'react';
import { View, Text } from 'react-native';
import { findIndex } from 'lodash';

import prdType from '../../config/productTypes.json';
import prdCondition from '../../config/productCondition.json';
import { secondaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingBottom: 16,
    marginHorizontal: 20,
    flexDirection: 'row',
  },
  label: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  content: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#000000',
  },
};

const getProductTypeText = (type) => {
  const idx = findIndex(prdType, o => o.value === type.toLowerCase());
  if (idx >= 0) {
    return prdType[idx].label;
  }
  return 'Unknown';
};

const getProductConditionText = (condition) => {
  const idx = findIndex(prdCondition, o => o.value === condition);
  if (idx >= 0) {
    return prdCondition[idx].label;
  }
  return 'Unknown';
};

export default ({ type, condition }) => {
  const typeText = getProductTypeText(type);
  return (
    <View style={styles.wrapper}>
      <Text style={styles.label}>{typeText}</Text>
      {typeText === 'Physical Good' && (
        <Text style={styles.label}>
          {` - ${getProductConditionText(condition.toLowerCase())}`}
        </Text>
      )}
    </View>
  );
};
