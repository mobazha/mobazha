import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, Image } from 'react-native';

import { priceStyle } from '../commonStyles';
import { convertorsMap } from '../../selectors/currency';
import BuyNowButton from './FullButton';
import { minUnitAmountToBCH } from '../../utils/currency';
import { COINS } from '../../utils/coins';
import { formLabelColor, brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'white',
    flexDirection: 'row',
    alignItems: 'center',
    borderTopWidth: 1,
    borderColor: '#e8e8e8',
  },
  priceWrapper: {
    flex: 5,
    alignSelf: 'center',
    paddingLeft: 16,
  },
  localPrice: {
    fontSize: 18,
    lineHeight: 18,
    fontWeight: 'bold',
    fontStyle: 'normal',
    textAlign: 'left',
    marginBottom: 2,
  },
  buyButtonContainer: {
    flex: 3,
  },
  buyButton: {
    backgroundColor: '#00bf65',
    marginLeft: 0,
  },
  cryptoWrapper: {
    marginTop: 3,
    flexDirection: 'row',
  },
  icon: {
    width: 15,
    height: 15,
  },
  crypto: {
    marginLeft: 3,
    fontSize: 13,
    color: formLabelColor,
  },
  calculating: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
};

class PayBanner extends PureComponent {
  render() {
    const {
      amount, localLabelFromBCH, onPay, paymentMethod, disabled,
    } = this.props;

    return (
      <View style={styles.wrapper}>
        {amount ? (
          <View style={styles.priceWrapper}>
            <Text style={[priceStyle, styles.localPrice]}>
              {localLabelFromBCH(amount, paymentMethod)}
            </Text>
            <View style={styles.cryptoWrapper}>
              <Image style={styles.icon} source={COINS[paymentMethod].icon} resizeMode="cover" />
              <Text style={styles.crypto}>
                {`${minUnitAmountToBCH(amount, paymentMethod)} ${paymentMethod}`}
              </Text>
            </View>
          </View>
        ) : (
          <View style={styles.priceWrapper}>
            <Text style={styles.calculating}>{I18n.t('components.atoms.PayBanner.calculating')}</Text>
          </View>
        )}
        <View style={styles.buyButtonContainer}>
          <BuyNowButton
            disabled={disabled}
            wrapperStyle={styles.buyButton}
            title={I18n.t('components.atoms.PayBanner.PAY')}
            onPress={onPay}
          />
        </View>
      </View>
    );
  }
}

export default connect(convertorsMap)(PayBanner);
