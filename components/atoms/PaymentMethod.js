import React from 'react';
import { View, Text, TouchableWithoutFeedback, Image } from 'react-native';
import * as _ from 'lodash';

import InputGroup from './InputGroup';
import BuyWyre from '../molecules/BuyWyre';
import { primaryTextColor, foregroundColor, brandColor } from '../commonColors';

import { COINS } from '../../utils/coins';
import { EditIcon } from '../../utils/checkout';

import {I18n} from '../../langs/I18n';

const styles = {
  paymentMethodWrapper: {
    paddingVertical: 12,
  },
  emptyWallet: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 19,
    letterSpacing: 0,
    textAlign: 'left',
    color: 'rgb(82, 82, 82)',
    marginBottom: 6,
  },
  addFundButton: {
    width: '100%',
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    paddingVertical: 10,
    paddingHorizontal: 12,
  },
  addFundText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    textAlign: 'center',
  },
  paymentMethod: {
    paddingTop: 12,
    paddingBottom: 16,
    flexDirection: 'row',
    alignItems: 'center',
  },
  coinImage: {
    width: 20,
    height: 20,
    marginRight: 5,
  },
  coinName: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  feeLevel: {
    marginLeft: 8,
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
};

export default ({
  balance, feeLevel, paymentMethod, toPaymentMethod, toAddFund,
}) => {
  const coin = COINS[paymentMethod];
  const coinBalance = balance && balance[paymentMethod] && (
    balance[paymentMethod].confirmed
  );
  return (
    <InputGroup title={I18n.t('components.atoms.PaymentMethod.payment_method')} action={toPaymentMethod} actionTitle={EditIcon}>
      <TouchableWithoutFeedback onPress={toPaymentMethod}>
        <View style={styles.paymentMethod}>
          <Image source={coin.icon} style={styles.coinImage} />
          <Text style={styles.coinName}>{coin.label}</Text>
          <Text style={styles.feeLevel}>{feeLevel.label}</Text>
        </View>
      </TouchableWithoutFeedback>
      {coinBalance <= 0 && false && (
        <View style={styles.paymentMethodWrapper}>
          <Text style={styles.emptyWallet}>{I18n.t('components.atoms.PaymentMethod.wallet_empty')}</Text>
          <TouchableWithoutFeedback onPress={() => toAddFund(paymentMethod)}>
            <View style={styles.addFundButton}>
              <Text style={styles.addFundText}>{I18n.t('components.atoms.PaymentMethod.add_funds')}</Text>
            </View>
          </TouchableWithoutFeedback>
        </View>
      )}
      <BuyWyre isCheckout />
    </InputGroup>
  );
};
