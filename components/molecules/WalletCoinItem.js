import React, { PureComponent } from 'react';
import { Alert, View, Text, Image, TouchableWithoutFeedback } from 'react-native';
import { connect } from 'react-redux';
import Ionicons from 'react-native-vector-icons/Ionicons';
import BigNumber from 'bignumber.js';
import * as _ from 'lodash';

import { foregroundColor, staticLabelColor, brandColor } from '../commonColors';

import { minUnitAmountToBCH } from '../../utils/currency';
import { COINS } from '../../utils/coins';
import { convertorsMap } from '../../selectors/currency';
import { cellStyles } from '../../utils/styles';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
  },
  contentWrapper: {
  },
  content: {
    padding: 15,
    flexDirection: 'row',
    alignItems: 'center',
  },
  leftWrapper: {
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    paddingLeft: 8,
  },
  rightWrapper: {
    alignItems: 'flex-end',
    justifyContent: 'space-between',
  },
  icon: {
    width: 34,
    height: 34,
  },
  coinName: {
    fontSize: 15,
    fontWeight: 'bold',
    letterSpacing: 0,
    color: '#000000',
  },
  priceByLocalCurrency: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
  secondary: {
    fontSize: 13,
    letterSpacing: 0,
    marginTop: 2,
    color: staticLabelColor,
  },
  comingSoon: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: foregroundColor,
  },
  priceWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-end',
  },
  unconfirmedBalanceWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-end',
    marginRight: 5,
  },
  unconfirmedBalance: {
    fontSize: 13,
    color: staticLabelColor,
  },
  timerIcon: {
    marginHorizontal: 2,
  },
};

class WalletCoinItem extends PureComponent {
  handlePress = () => {
    const { coin, balance, onCoinSelected } = this.props;
    const { disabled } = COINS[coin];
    if (disabled || !balance) {
      Alert.alert(I18n.t('components.molecules.WalletCoinItem.coming_soon'));
    } else {
      onCoinSelected(coin);
    }
  }

  render() {
    const { balance = {}, coin, convertBalanceFromBCH, localLabelFromLocal } = this.props;
    const { label, icon, disabled } = COINS[coin];
    const { cBalance, pending } = convertBalanceFromBCH(balance, coin);

    const cAmount = BigNumber(balance.confirmed || '0');
    const ucAmount = BigNumber(balance.unconfirmed || '0');

    return (
      <View style={styles.wrapper}>
        <TouchableWithoutFeedback style={styles.contentWrapper} onPress={this.handlePress} >
          <View style={styles.content}>
            <Image style={styles.icon} source={icon} resizeMode="cover" />
            <View style={styles.leftWrapper}>
              <Text style={styles.coinName}>{label}</Text>
              <Text style={styles.secondary}>{coin}</Text>
            </View>
            <View style={{ flex: 1 }} />
            <View style={styles.rightWrapper}>
              <View style={styles.priceWrapper}>
                {pending > 0 && (
                  <View style={styles.unconfirmedBalanceWrapper}>
                    <Text style={styles.unconfirmedBalance}>(</Text>
                    <Ionicons style={styles.timerIcon} name="md-time" size={13} color={staticLabelColor} />
                    <Text style={styles.unconfirmedBalance}>
                      {localLabelFromLocal(pending)})
                    </Text>
                  </View>
                )}
                <Text style={styles.priceByLocalCurrency}>
                  {localLabelFromLocal(cBalance)}
                </Text>
              </View>
              <Text style={styles.secondary}>
                {(disabled || !balance) ? (
                  I18n.t('components.molecules.WalletCoinItem.coming_soon')
                ) : (
                    `${minUnitAmountToBCH(cAmount.plus(ucAmount), coin)} ${coin}`
                  )}
              </Text>
            </View>
          </View>
        </TouchableWithoutFeedback>
        <View style={cellStyles.separatorContainer}>
          <View style={cellStyles.separator} />
        </View>
      </View>
    );
  }
}

export default connect(convertorsMap)(WalletCoinItem);
