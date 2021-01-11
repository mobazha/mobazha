import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import BigNumber from 'bignumber.js';

import { brandColor, foregroundColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: brandColor,
    paddingTop: 10,
    height: 70,
    paddingBottom: 20,
    justifyContent: 'center',
    alignItems: 'center',
  },
  balance: {
    fontSize: 30,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: 'white',
  },
  pendingBalanceWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
  },
  pendingBalance: {
    fontSize: 14,
    color: foregroundColor,
  },
  timerIcon: {
    marginHorizontal: 2,
  },
};

class Balance extends PureComponent {
  render() {
    const {
      balance, convertBalanceFromBCH, localLabelFromLocal,
    } = this.props;
    const { totalBalance, pendingBalance } = Object.keys(balance).reduce((prev, coin) => {
      const { cBalance, pending } = convertBalanceFromBCH(balance[coin], coin);
      return {
        totalBalance: BigNumber(prev.totalBalance).plus(BigNumber(cBalance)),
        pendingBalance: BigNumber(prev.pendingBalance).plus(BigNumber(pending)),
      };
    }, { totalBalance: 0, pendingBalance: 0 });
    return (
      <View style={styles.wrapper}>
        <Text style={styles.balance}>{localLabelFromLocal(totalBalance)}</Text>
        {pendingBalance > 0 && (
          <View style={styles.pendingBalanceWrapper}>
            <Text style={styles.pendingBalance}>(</Text>
            <Ionicons style={styles.timerIcon} name="md-time" size={13} color={foregroundColor} />
            <Text style={styles.pendingBalance}>
              {I18n.t('components.organism.Balance.unconfirmed', {balance: localLabelFromLocal(pendingBalance)})})
            </Text>
          </View>
        )}
      </View>
    );
  }
}

export default connect(convertorsMap)(Balance);
