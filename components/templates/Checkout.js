import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, ScrollView, Text, TouchableWithoutFeedback, Linking } from 'react-native';
import { isEmpty, isEqual, get } from 'lodash';
import { withNavigation } from 'react-navigation';

import CheckoutSummary from '../organism/CheckoutSummary';
import CheckoutHeader from '../organism/CheckoutHeader';
import PaymentMethod from '../atoms/PaymentMethod';
import Moderator from '../organism/Moderator';

import InputGroup from '../atoms/InputGroup';
import { primaryTextColor, formLabelColor, foregroundColor, warningColor } from '../commonColors';
import { COINS } from '../../utils/coins';
import { setPaymentMethod } from '../../reducers/appstate';
import { EditIcon, InfoIcon } from '../../utils/checkout';
import CheckoutNote from '../molecules/CheckoutNote';
import { parseCountryName } from '../../utils/string';
import CheckBox from '../atoms/CheckBox';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  addrWrapper: {
    paddingTop: 12,
    paddingBottom: 16,
  },
  addrName: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  addrLine: {
    fontSize: 15,
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  moderatorText: {
    fontSize: 15,
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  moderatorNotAvailable: {
    paddingVertical: 10,
    fontSize: 15,
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: warningColor,
  },
  emptyAddress: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 19,
    letterSpacing: 0,
    textAlign: 'left',
    color: 'black',
  },
  cannotShip: {
    color: '#ff3b30',
  },
  fullButton: {
    marginTop: 16,
    width: '100%',
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    paddingVertical: 10,
    paddingHorizontal: 12,
  },
  fullText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    textAlign: 'center',
  },
  optionWrapper: {
    paddingBottom: 16,
  },
};

class Checkout extends PureComponent {
  constructor(props) {
    super(props);
    const { quantity, setPaymentMethod } = props;
    this.state = {
      quantity,
      isProtected: true,
    };

    setPaymentMethod({
      paymentMethod: this.getAdjustedPaymentMethod(),
    });
  }

  getAdjustedPaymentMethod = () => {
    const {
      listing: {
        listing: {
          metadata: { acceptedCurrencies },
        },
      },
      paymentMethod,
    } = this.props;

    if (acceptedCurrencies.includes(paymentMethod)) {
      return paymentMethod;
    }
    return acceptedCurrencies.find(c => COINS[c]);
  };

  handleGoToAddAddress = () => {
    this.props.navigation.navigate('EditShippingAddress', {
      shippingIndex: -1,
    });
  }

  handleProtectedToggle = () => {
    const { isProtected } = this.state;
    if (isProtected) {
      eventTracker.trackEvent('Checkout-DisabledPaymentProtection');
    } else {
      eventTracker.trackEvent('Checkout-EnabledPaymentProtection');
    }
    this.setState({ isProtected: !isProtected });
  }

  handleOpenHelp = () => {
    const url = 'https://mobazha.info/faq';
    Linking.canOpenURL(url)
      .then(supported => supported && Linking.openURL(url))
      .catch(() => {});
  }

  handleGoToModerator = () => {
    const { navigation, moderator } = this.props;
    eventTracker.trackEvent('Checkout-ChangingModerator');
    navigation.push('CheckoutModeratorDetails', { moderator });
  }

  renderCurrentShippingAddress = () => {
    const { listing, shippingAddress: addr, toShippingAddress } = this.props;
    const { shippingOptions } = listing.listing;

    if (isEmpty(addr)) {
      return (
        <TouchableWithoutFeedback onPress={this.handleGoToAddAddress}>
          <View style={styles.addrWrapper}>
            <Text style={[styles.emptyAddress, styles.cannotShip]}>{I18n.t('components.templates.Checkout.address_required')}</Text>
            <View style={styles.fullButton}>
              <Text style={styles.fullText}>{I18n.t('components.templates.Checkout.new_address')}</Text>
            </View>
          </View>
        </TouchableWithoutFeedback>
      );
    } else {
      const regionChecker = op => op.regions.includes('ALL') || op.regions.includes(addr.country);
      const validOptions = shippingOptions && shippingOptions.filter(regionChecker);

      if (!validOptions || validOptions.length === 0) {
        return (
          <TouchableWithoutFeedback onPress={toShippingAddress}>
            <View style={styles.addrWrapper}>
              <Text style={[styles.emptyAddress, styles.cannotShip]}>
              {I18n.t('components.templates.Checkout.cannot_ship')}
              </Text>
            </View>
          </TouchableWithoutFeedback>
        );
      }

      return (
        <TouchableWithoutFeedback onPress={toShippingAddress}>
          <View style={styles.addrWrapper}>
            <Text style={styles.addrName}>{addr.name}</Text>
            <Text style={styles.addrLine}>
              {`${addr.addressLineOne} ${addr.addressLineTwo}`}
            </Text>
            <Text style={styles.addrLine}>
              {`${addr.city}`}
              {addr.state ? `, ${addr.state}` : ''}
              {addr.postalCode ? ` ${addr.postalCode}` : ''}
            </Text>
            <Text style={styles.addrLine}>
              {parseCountryName(addr.country)}
            </Text>
          </View>
        </TouchableWithoutFeedback>
      );
    }
  }

  render() {
    const {
      listing,
      coin,
      shippingAddress,
      shippingOption,
      balance,
      toPaymentMethod,
      toAddFund,
      feeLevel,
      toModerators,
      moderator,
      setCheckoutObject,
      loadingModerators,
      paymentMethod,
      toShippingAddress,
      selectedCoupon,
      onChangeCoupon,
      combo,
    } = this.props;
    const { quantity, isProtected } = this.state;

    const skus = get(listing, 'listing.item.skus', []);
    const { contractType, acceptedCurrencies } = get(listing, 'listing.metadata', {});
    const mergedCombo = combo || (skus.length === 1 && skus[0].variantCombo);
    const variantInfo = skus ? skus.find(o => isEqual(o.variantCombo, mergedCombo)) : [];

    return (
      <ScrollView>
        <CheckoutHeader
          listing={listing}
          variantInfo={variantInfo}
          quantity={quantity}
          onChange={quantity => this.setState({ quantity })}
        />
        <CheckoutSummary
          listing={listing}
          combo={mergedCombo}
          selectedCoupon={selectedCoupon}
          quantity={quantity}
          variantInfo={variantInfo}
          paymentMethod={paymentMethod}
          feeLevel={feeLevel}
          moderator={isProtected && moderator}
          productType={contractType}
          shippingOption={shippingOption}
          shippingAddress={shippingAddress}
          toShippingAddress={toShippingAddress}
          toPaymentMethod={() => {
            toPaymentMethod(acceptedCurrencies);
          }}
          setCheckoutObject={setCheckoutObject}
          onChangeCoupon={onChangeCoupon}
        />
        {contractType === 'PHYSICAL_GOOD' && (
          <InputGroup
            title={I18n.t('components.templates.Checkout.shipping')}
            actionTitle={EditIcon}
            action={toShippingAddress}
          >
            {this.renderCurrentShippingAddress()}
          </InputGroup>
        )}
        <PaymentMethod
          paymentMethod={paymentMethod}
          feeLevel={feeLevel}
          balance={balance}
          toPaymentMethod={() => {
            toPaymentMethod(acceptedCurrencies);
          }}
          toAddFund={toAddFund}
          coin={coin}
        />
        {!loadingModerators && (
          <InputGroup
            title={I18n.t('components.templates.Checkout.payment_protection')}
            actionTitle={InfoIcon}
            action={this.handleOpenHelp}
          >
            <View style={styles.optionWrapper}>
              {!isEmpty(moderator) ? (
                <CheckBox
                  checked={isProtected}
                  title={<Text>{I18n.t('components.templates.Checkout.protect_up_to')} <Text style={{ fontWeight: 'bold' }}>{I18n.t('components.templates.Checkout.protect_days')}</Text></Text>}
                  onPress={this.handleProtectedToggle}
                />
              ) : (
                <Text style={styles.moderatorNotAvailable}>{I18n.t('components.templates.Checkout.moderator_not_available')}</Text>
              )}
              {(isProtected && !isEmpty(moderator)) ? (
                [
                  <TouchableWithoutFeedback onPress={this.handleGoToModerator}>
                    <View>
                      <Moderator peerID={moderator} />
                    </View>
                  </TouchableWithoutFeedback>,
                  <TouchableWithoutFeedback onPress={toModerators}>
                    <View style={styles.fullButton}>
                      <Text style={styles.fullText}>{I18n.t('components.templates.Checkout.change_moderator')}</Text>
                    </View>
                  </TouchableWithoutFeedback>,
                ]
              ) : (
                <Text style={styles.moderatorText}>
                  {I18n.t('components.templates.Checkout.no_moderator_description')}                  
                </Text>
              )}
            </View>

          </InputGroup>
        )}
        <CheckoutNote />
      </ScrollView>
    );
  }
}

const mapStateToProps = state => ({
  moderator: state.appstate.moderator,
});

const mapDispatchToProps = {
  setPaymentMethod,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
)(Checkout));
