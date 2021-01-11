import React from 'react';
import { View, Text, TouchableWithoutFeedback, Linking, Platform } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { brandColor, staticLabelColor } from '../commonColors';
import { minUnitAmountToBCH } from '../../utils/currency';
import { transactionLinkDict } from '../../utils/coins';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingTop: 19,
    paddingBottom: Platform.OS === 'ios' ? 16 : 19,
    marginHorizontal: 15,
  },
  mainContent: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-start',
  },
  amount: {
    fontSize: 14,
    fontWeight: 'bold',
    letterSpacing: 0,
    textAlign: 'left',
    color: brandColor,
    marginRight: 3,
    height: 22,
    lineHeight: 22,
  },
  coinIcon: {
    width: 16,
    height: 16,
  },
  time: {
    fontSize: 12,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'right',
    height: 22,
    lineHeight: 22,
    color: staticLabelColor,
  },
  sendingAmountColor: {
    color: '#eb0404',
  },
  borderStyle: {
    borderBottomWidth: 1,
    borderBottomColor: '#e8e8e8',
  },
  cryptoAmount: {
    fontSize: 12,
    height: 22,
    lineHeight: 22,
    color: staticLabelColor,
  },
  timerIcon: {
    marginHorizontal: 2,
    lineHeight: 22,
  },
};

export default ({
  amount, coin, currencySymbol, time, noBorder, id, status, valueInCrypto,
}) => {
  const cryptoInDecimal = minUnitAmountToBCH(Math.abs(valueInCrypto || 0), coin).toFixed(8);
  return (
    <TouchableWithoutFeedback
      onPress={() => {
        Linking.openURL(transactionLinkDict(id)[coin]);
      }}
    >
      <View style={[styles.wrapper, noBorder ? {} : styles.borderStyle]}>
        <View style={styles.mainContent}>
          {status === 'UNCONFIRMED' && <Ionicons size={13} color={staticLabelColor} style={styles.timerIcon} name="md-time" />}
          <Text style={[styles.amount, amount >= 0 ? {} : styles.sendingAmountColor]}>
            {`${amount >= 0 ? '+' : '-'}${currencySymbol}${Math.abs(amount).toFixed(2)}`}
          </Text>
          <Text style={styles.cryptoAmount}>
            {cryptoInDecimal} {coin}
          </Text>
        </View>
        <Text style={styles.time}>{time}</Text>
      </View>
    </TouchableWithoutFeedback>
  );
};
