import React, { PureComponent } from 'react';
import { View, Text, Image } from 'react-native';
import { connect } from 'react-redux';

import ProductSection from '../atoms/ProductSection';
import { primaryTextColor, warningColor } from '../commonColors';

import { exchangeCurrency, removeZeros } from '../../utils/currency';
import { getRenderingCoins } from '../../utils/coins';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    flex: 1,
    margin: 4,
  },
  coinImage: {
    width: 20,
    height: 20,
    marginRight: 6,
  },
  label: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  content: {
    flex: 1,
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  leftPart: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'flex-start',
  },
  explanation: {
    textAlign: 'right',
    fontSize: 16,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: warningColor,
  },
};

class ListingPaymentOptions extends PureComponent {
  renderPaymentMethod = (coin) => {
    const { listing, convertBetweenCurrencies } = this.props;
    if (!listing || !listing.item) {
      return null;
    }

    const { bigPrice: price, priceCurrency = {} } = listing.item;
    return (
      <View key={coin.value} style={[styles.wrapper, { justifyContent: 'space-between' }]}>
        <View style={styles.content}>
          <View style={[styles.leftPart, { opacity: coin.disabled ? 0.4 : 1 }]}>
            <Image source={coin.icon} style={styles.coinImage} />
            <Text style={[styles.label, coin.disabled && { fontStyle: 'italic' }]}>
              {coin.disabled ? (
                I18n.t('components.molecules.ListingPaymentOptions.not_accepted')
               ) : (
                `${removeZeros(convertBetweenCurrencies(price, priceCurrency.code, coin.value).toFixed(8))} ${coin.value}`
              )}
            </Text>
          </View>
        </View>
      </View>
    );
  }

  render() {
    const { acceptedCurrencies } = this.props.listing.metadata;
    const renderingCoins = getRenderingCoins(acceptedCurrencies);
    return (
      <ProductSection title={I18n.t('components.molecules.ListingPaymentOptions.payment_options')}>
        {renderingCoins.map(this.renderPaymentMethod)}
      </ProductSection>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(ListingPaymentOptions);
