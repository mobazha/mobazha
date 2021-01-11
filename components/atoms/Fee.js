import React from 'react';
import { View, Text } from 'react-native';
import { connect } from 'react-redux';
import * as _ from 'lodash';
import FontAwesome from 'react-native-vector-icons/FontAwesome';

import { getCurrencySymbol } from '../../utils/currency';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    flexDirection: 'row',
    alignItems: 'center',
  },
};

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

const Fee = ({
  moderatorInfo, verbose, convertCurrency, style,
}) => {
  if (!moderatorInfo) {
    return null;
  }

  const {
    feeType, percentage, fixedFee,
  } = moderatorInfo;

  const verboseText = verbose ? ' service fee' : '';

  let serviceFee;
  switch (feeType) {
    case 'FIXED': {
      const symbol = getCurrencySymbol(fixedFee.currencyCode);
      serviceFee = ` ${symbol}${convertCurrency(fixedFee.amount, fixedFee.currencyCode).toFixed(2)}${verboseText}`;
      break;
    }
    case 'PERCENTAGE':
      serviceFee = ` ${percentage}%${verboseText}`;
      break;
    default: {
      const symbol = getCurrencySymbol(fixedFee.currencyCode);
      serviceFee = ` ${percentage}% + ${symbol}${convertCurrency(fixedFee.amount, fixedFee.currencyCode).toFixed(2)}${verboseText}`;
    }
  }

  return (
    <View style={styles.container}>
      <FontAwesome name="dollar" size={14} color="#8cd985" />
      <Text style={style}>{serviceFee}</Text>
    </View>
  );
};

export default connect(mapStateToProps)(Fee);
