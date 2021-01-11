import React, { PureComponent } from 'react';
import { Image, View, Text } from 'react-native';
import { connect } from 'react-redux';
import BigNumber from 'bignumber.js';
import * as _ from 'lodash';

import { brandColor, foregroundColor, staticLabelColor } from '../commonColors';
import { minUnitAmountToBCH } from '../../utils/currency';
import { convertorsMap } from '../../selectors/currency';
import { COINS } from '../../utils/coins';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    justifyContent: 'center',
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: '#e8e8e8',
    paddingBottom: 25,
    marginHorizontal: 15,
  },
  balance: {
    fontSize: 30,
    fontWeight: 'bold',
    letterSpacing: 0,
    textAlign: 'left',
    color: brandColor,
  },
  pending: {
    fontSize: 14,
    fontWeight: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: staticLabelColor,
  },
  icon: {
    width: 38,
    height: 38,
    marginVertical: 10,
  },
};

class CoinBalance extends PureComponent {
  render() {
    const {
      localLabelFromLocal, balance, coin, convertBalanceFromBCH,
    } = this.props;
    const { icon } = COINS[coin];
    const { cBalance } = convertBalanceFromBCH(balance, coin);
    const cAmount = BigNumber(balance.confirmed || 0);
    const ucAmount = BigNumber(balance.unconfirmed || 0);
    const coinBalance = minUnitAmountToBCH(cAmount.plus(ucAmount), coin);
    return (
      <View style={styles.wrapper}>
        <Image style={styles.icon} source={icon} resizeMode="cover" />
        <Text style={styles.balance}>{localLabelFromLocal(cBalance)}</Text>
        <Text style={styles.pending}>{`${coinBalance} ${coin}`}</Text>
      </View>
    );
  }
}

export default connect(convertorsMap)(CoinBalance);
