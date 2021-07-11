import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, Platform, Alert } from 'react-native';
import * as _ from 'lodash';

import { greenColor } from '../commonColors';
import { priceStyle } from '../commonStyles';
import { convertorsMap } from '../../selectors/currency';
import BuyNowButton from './FullButton';
import UnavailableButton from './UnavailableButton';
import AverageRating from '../organism/AverageRating';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'white',
    flexDirection: 'row',
    alignItems: 'center',
    borderTopWidth: 1,
    borderColor: '#e8e8e8',
  },
  priceRatingsWrapper: {
    flex: 5,
  },
  priceWrapper: {
    justifyContent: 'flex-start',
    flexDirection: 'row',
    alignItems: 'flex-end',
    marginBottom: 2,
    marginLeft: 16,
  },
  price: {
    fontSize: 18,
    lineHeight: 18,
    fontWeight: 'bold',
    fontStyle: 'normal',
    textAlign: 'center',
  },
  shipping: {
    fontSize: 11,
    lineHeight: 17,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: greenColor,
  },
  freeShippingTextWrapper: {
    paddingHorizontal: 8,
    alignSelf: 'center',
  },
  freeShipping: {
    lineHeight: 17,
    fontSize: 11,
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#00BF65',
  },
  buyButtonContainer: {
    flex: 3,
  },
  buyButton: {
    backgroundColor: '#00bf65',
    marginLeft: 0,
  },
  ratings: {
    marginLeft: 16,
  },
};

class ProductPrice extends PureComponent {
  onPressUnavailable = () => {
    Alert.alert(I18n.t('components.atoms.ProductPrice.Digital_unavailable'));
  }

  renderShippingPrice = () => {
    const {
      listing, shippingOption, localLabelFromBCH, localDecimalPointsIfCrypto,
    } = this.props;

    if (!shippingOption || shippingOption.price == null) {
      return null;
    }

    if (shippingOption.price === 0) {
      return (
        <View style={styles.freeShippingTextWrapper}>
          <Text style={[styles.freeShipping]}>{I18n.t('components.atoms.ProductPrice.free_shipping')}</Text>
        </View>
      );
    } else {
      const currency = _.get(listing, 'item.priceCurrency.code');
      const localLabel = localLabelFromBCH(shippingOption.price, currency);
      if (localLabel === 'calculating...') {
        return null;
      }
      return (
        <Text style={styles.shipping}>
          {(localDecimalPointsIfCrypto ? ' + ' : ` +${localLabel} `) + I18n.t('components.atoms.ProductPrice.shipping')}
        </Text>
      );
    }
  }

  render() {
    const {
      listing, localLabelFromBCH, onBuy, peerID, slug,
    } = this.props;

    const { item = {}, metadata = {} } = listing;
    const amount = item.bigPrice;
    const currency = _.get(item, 'priceCurrency.code');
    const productType = metadata.contractType;

    return (
      <View style={styles.wrapper}>
        <View style={styles.priceRatingsWrapper}>
          <View style={styles.priceWrapper}>
            <Text style={[priceStyle, styles.price]} numberOfLines={1}>
              {localLabelFromBCH(amount, currency)}
            </Text>
            {this.renderShippingPrice()}
          </View>
          <View style={styles.ratings}>
            <AverageRating peerID={peerID} slug={slug} />
          </View>
        </View>
        <View style={styles.buyButtonContainer}>
          {productType === 'DIGITAL_GOOD' && Platform.OS === 'ios' ? (
            <UnavailableButton onPress={this.onPressUnavailable} />
          ) : (
              <BuyNowButton
                wrapperStyle={styles.buyButton}
                title={I18n.t('components.atoms.ProductPrice.BUY_NOW')}
                onPress={onBuy}
              />
            )}
        </View>
      </View>
    );
  }
}

export default connect(convertorsMap)(ProductPrice);
