/* eslint-disable no-mixed-operators */
import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { TouchableWithoutFeedback, Text, View, Image, Linking, Alert } from 'react-native';
import * as _ from 'lodash';
import deepEqual from 'deep-equal';
import decode from 'unescape';
import Dash from 'react-native-dash';
import { withNavigation } from 'react-navigation';
import BigNumber from 'bignumber.js';

import InputGroup from '../atoms/InputGroup';
import ListingTitlePrice from '../atoms/ListingTitlePrice';

import { generateCheckoutObject, parseVariantInfo } from '../../utils/checkout';
import { getCheckoutBreakdown } from '../../api/checkout';
import { primaryTextColor, brandColor, formLabelColor, borderColor, linkTextColor, warningColor } from '../commonColors';
import { priceStyle } from '../commonStyles';
import { convertorsMap } from '../../selectors/currency';

import { COINS } from '../../utils/coins';
import { getListingActualPrice } from '../../utils/stockManage';
import { estimateFee } from '../../api/wallet';
import CouponApplyModal from '../templates/CouponApplyModal';

import {I18n} from '../../langs/I18n';

const FEE_MAX_PERCENT = 0.25;

const generateCheckoutObjectFromProps = (props) => {
  const {
    quantity,
    shippingOption,
    productType,
    shippingAddress,
    variantInfo,
    moderator,
    paymentMethod,
    checkoutNote,
    selectedCoupon,
  } = props;

  const { options } = props.listing.listing.item;
  const listingHash = props.listing.hash;

  const variantData = parseVariantInfo(variantInfo, options);
  const shippingAddressParam = (productType === 'PHYSICAL_GOOD' && shippingAddress) || {};
  const shippingOptionParam = (productType === 'PHYSICAL_GOOD' && shippingOption) || {};
  return generateCheckoutObject(
    productType,
    shippingAddressParam,
    listingHash,
    quantity,
    variantData,
    shippingOptionParam,
    moderator,
    paymentMethod,
    checkoutNote,
    selectedCoupon,
  );
};

const styles = {
  optionWrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    flex: 1,
    paddingVertical: 12,
  },
  optionLabelWrapper: {
    maxWidth: 250,
    flexDirection: 'row',
    alignItems: 'center',
    paddingRight: 15,
  },
  optionLabel: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    flex: 1,
  },
  changeText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: linkTextColor,
  },
  paymentMethod: {
    paddingVertical: 12,
    flexDirection: 'row',
    alignItems: 'flex-start',
    flex: 1,
    justifyContent: 'space-between',
  },
  paymentLabel: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  coinImage: {
    width: 20,
    height: 20,
  },
  feeLevel: {
    marginLeft: 5,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  totalWrapper: {
    paddingVertical: 12,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  totalLabel: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  totalValue: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
  option: {
    fontSize: 13,
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
    marginBottom: 8,
  },
  feeAlertContainer: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginBottom: 8,
  },
  feeAlertDescription: {
    marginRight: 4,
    color: warningColor,
    flex: 1,
  },
  feeAlertLearnMore: {
    color: warningColor,
    textDecorationLine: 'underline',
  },
  couponWrapper: {
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  couponInfo: {
    flexDirection: 'row',
  },
  couponLabel: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    marginRight: 8,
    color: 'black',
  },
  couponLinkText: {
    fontSize: 15,
    color: linkTextColor,
  },
  couponDiscountText: {
    color: warningColor,
  },
};

class CheckoutSummary extends PureComponent {
  constructor(props) {
    super(props);

    this.bouncedGetEstimation = _.debounce(this.getEstimation, 1000);
  }

  state = {
    showCouponModal: false,
  }

  UNSAFE_componentWillMount() {
    const checkoutObj = generateCheckoutObjectFromProps(this.props);
    this.bouncedGetEstimation(checkoutObj, this.props);
  }

  UNSAFE_componentWillReceiveProps(nextProps) {
    if (this.props.paymentMethod !== nextProps.paymentMethod) {
      this.setState({ price: undefined, shippingPrice: undefined, estimatedFee: undefined });
    }

    const { moderator: _moderator, ...originObj } = generateCheckoutObjectFromProps(this.props);
    const { moderator: _nextModerator, ...nextObj } = generateCheckoutObjectFromProps(nextProps);
    if (!deepEqual(originObj, nextObj) || !deepEqual(this.props.feeLevel.fee, nextProps.feeLevel.fee)) {
      this.bouncedGetEstimation(nextObj, nextProps);
    }
  }

  getEstimation = async (checkoutObject, nextProps) => {
    const roughPrice = this.getRoughEstimation(nextProps);
    const {
      username, password, setCheckoutObject, shippingAddress, productType, feeLevel, paymentMethod,
    } = nextProps;
    if (_.isEmpty(shippingAddress) && productType === 'PHYSICAL_GOOD') {
      this.setState({ price: roughPrice, isRough: true, estimatedFee: undefined }, () => {
        setCheckoutObject({
          checkoutObject,
          price: roughPrice.plus(this.getCurrentFee()),
          fee: this.getCurrentFee(),
        });
      });
      return;
    }

    this.setState({ price: roughPrice, isRough: true });
    setCheckoutObject({
      checkoutObject,
      price: roughPrice.plus(this.getCurrentFee()),
      fee: this.getCurrentFee(),
    });

    // estimateFee api fails when the price is not an integer, so we apply Math.ceil here
    const estimatedFee = await estimateFee(paymentMethod, Math.ceil(roughPrice), feeLevel.value);
    this.setState({ estimatedFee: estimatedFee.amount }, () => {
      setCheckoutObject({
        checkoutObject,
        price: roughPrice.plus(this.getCurrentFee()),
        fee: this.getCurrentFee(),
      });
    });

    getCheckoutBreakdown(username, password, checkoutObject).then(async (response) => {
      if (_.hasIn(response, 'success')) {
        // failure... do nothing, just rely on estimated value in this case
      } else {
        const { totalPrice: price, shippingPrice, couponPrice } = response;
        this.setState({ price: BigNumber(price), shippingPrice, couponPrice, isRough: false });
        setCheckoutObject({
          checkoutObject,
          price: BigNumber(price).plus(this.getCurrentFee()),
          fee: this.getCurrentFee(),
        });

        const estimatedFee = await estimateFee(paymentMethod, price, feeLevel.value);
        this.setState({ estimatedFee: estimatedFee.amount }, () => {
          setCheckoutObject({
            checkoutObject,
            price: BigNumber(price).plus(this.getCurrentFee()),
            fee: this.getCurrentFee(),
          });
        });
      }
    });
  }

  getShippingTotalPrice = (props, donotConvertCurrency = false) => {
    const {
      productType, listing, quantity, shippingOption, shippingAddress, convertCurrency,
    } = props;

    if (productType !== 'PHYSICAL_GOOD') {
      return 0;
    }

    const { code: currency } = listing.listing.item.priceCurrency;
    if (!shippingOption || _.isEmpty(shippingAddress)) {
      return 0;
    }

    const totalPrice = (+shippingOption.bigPrice + +shippingOption.bigAdditionalItemPrice * (quantity - 1)) || 0;
    if (donotConvertCurrency) {
      return BigNumber(totalPrice);
    }

    return convertCurrency(totalPrice, currency);
  }

  getRoughEstimation = (props) => {
    const {
      quantity, paymentMethod, convertCurrencyToBCH,
    } = props;
    const localPrice = getListingActualPrice(props)
      .multipliedBy(quantity)
      .plus(this.getShippingTotalPrice(props));
    return convertCurrencyToBCH(localPrice, paymentMethod);
  }

  getCurrentFee = () => {
    const { feeLevel } = this.props;
    const { estimatedFee } = this.state || {};
    return estimatedFee || _.get(feeLevel, 'fee.amount', 0) * 400 || 0;
  }

  handleGoToAddCoupons = () => {
    this.props.navigation.navigate('AddListingCoupon');
  };

  handleOpenHelp = () => {
    const url = 'https://mobazha.info/faq';
    Linking.canOpenURL(url)
      .then(supported => supported && Linking.openURL(url))
      .catch(() => {});
  }

  hideCouponModal = () => {
    this.setState({ showCouponModal: false });
  }

  showCouponModal = () => {
    this.setState({ showCouponModal: true });
  }

  handleCouponSelect = (coupon) => {
    this.props.onChangeCoupon(coupon);
  }

  handleRemoveCoupon = () => {
    Alert.alert(I18n.t('components.organism.CheckoutSummary.remove_coupon'), I18n.t('components.organism.CheckoutSummary.remove_coupon_description'), [
      { text: I18n.t('components.organism.CheckoutSummary.cancel') },
      {
        text: I18n.t('components.organism.CheckoutSummary.remove'),
        onPress: () => {
          this.props.onChangeCoupon({});
        },
      },
    ]);
  }

  renderOptions() {
    const { combo, listing, quantity } = this.props;
    const { options } = listing.listing.item;

    return (
      <View>
        {combo && combo.map((val, idx) => (
          <Text style={styles.option} key={`option_${options[idx].variants[val].name}`}>
            {`${decode(options[idx].name)}: ${decode(options[idx].variants[val].name)}`}
          </Text>
        ))}
        <Text style={styles.option}>
        {I18n.t('components.organism.CheckoutSummary.quantity')}: {quantity}
        </Text>
      </View>
    );
  }

  renderCouponOption() {
    const {
      listing,
      selectedCoupon = {},
      convertCurrency,
      localSymbol,
      quantity,
      paymentMethod,
      localLabelFromBCH,
    } = this.props;
    const { isRough, couponPrice } = this.state;
    const coupons = _.get(listing, 'listing.coupons', []);
    const currency = _.get(listing, 'listing.metadata.pricingCurrency');
    const coupon = !_.isEmpty(selectedCoupon) && coupons.find(coupon =>
      (coupon.discountCode && coupon.discountCode === selectedCoupon.couponCode) ||
      coupon.hash === selectedCoupon.couponHash,
    );
    return (
      <View style={styles.couponWrapper}>
        <View style={styles.couponInfo}>
          <Text style={styles.couponLabel}>
          {I18n.t('components.organism.CheckoutSummary.coupon_info', {info:coupon ? selectedCoupon.couponCode : I18n.t('components.organism.CheckoutSummary.none_select')})}
          </Text>
          <TouchableWithoutFeedback onPress={coupon ? this.handleRemoveCoupon : this.showCouponModal}>
            <Text style={styles.couponLinkText}>{coupon ? I18n.t('components.organism.CheckoutSummary.remove') : I18n.t('components.organism.CheckoutSummary.add_link')}</Text>
          </TouchableWithoutFeedback>
        </View>
        {coupon && (
          <Text style={styles.couponDiscountText}>
            {isRough ? (
              coupon.percentDiscount ? `-${coupon.percentDiscount}%` : `-${localSymbol}${(convertCurrency(coupon.priceDiscount, currency) * quantity).toFixed(2)}`
            ) : (
              localLabelFromBCH(couponPrice, paymentMethod)
            )}
          </Text>
        )}
      </View>
    );
  }

  render() {
    const {
      listing,
      paymentMethod,
      toPaymentMethod,
      localLabelFromBCH,
      productType,
      toShippingAddress,
      shippingOption,
      shippingAddress,
      quantity,
    } = this.props;

    const { title, priceCurrency = {} } = listing.listing.item;
    const { code: currency } = priceCurrency;

    const { price, shippingPrice, showCouponModal, isRough } = this.state;
    const totalPrice = price && BigNumber(price).plus(this.getCurrentFee());
    return (
      <InputGroup
        title={I18n.t('components.organism.CheckoutSummary.summary')}
        // actionTitle="Add coupon"
        // action={this.handleGoToAddCoupons}
      >
        <ListingTitlePrice
          title={decode(title)}
          price={getListingActualPrice(this.props, true)}
          currency={currency}
          quantity={quantity}
        />
        {this.renderOptions()}
        {productType === 'PHYSICAL_GOOD' && !_.isEmpty(shippingAddress) && !_.isEmpty(shippingOption) && (
          <TouchableWithoutFeedback onPress={toShippingAddress}>
            <View style={styles.optionWrapper} >
              <View style={styles.optionLabelWrapper}>
                <Text style={styles.optionLabel}>
                  {`${decode(shippingOption.name)} ${decode(shippingOption.service)}  `}
                  <Text style={styles.changeText}>{I18n.t('components.organism.CheckoutSummary.change')}</Text>
                </Text>
              </View>
              <Text style={priceStyle}>
                {isRough ? (
                  !shippingOption.bigPrice ? I18n.t('components.organism.CheckoutSummary.free'): localLabelFromBCH(this.getShippingTotalPrice(this.props, true), currency)
                ) : (
                  !shippingPrice ? I18n.t('components.organism.CheckoutSummary.free') : localLabelFromBCH(shippingPrice, paymentMethod)
                )}
              </Text>
            </View>
          </TouchableWithoutFeedback>
        )}
        {this.renderCouponOption()}
        <TouchableWithoutFeedback onPress={toPaymentMethod}>
          <View style={styles.paymentMethod}>
            <View style={styles.paymentLabel}>
              <Image source={COINS[paymentMethod].icon} style={styles.coinImage} />
                <Text style={styles.feeLevel}>{I18n.t('components.organism.CheckoutSummary.network_fee')}&nbsp;&nbsp;</Text>
              <Text style={styles.changeText}>{I18n.t('components.organism.CheckoutSummary.change')}</Text>
            </View>
            <Text style={priceStyle}>
              {localLabelFromBCH(this.getCurrentFee(), paymentMethod)}
            </Text>
          </View>
        </TouchableWithoutFeedback>
        {totalPrice && (totalPrice.multipliedBy(FEE_MAX_PERCENT).isLessThan(this.getCurrentFee())) && (
          <View style={styles.feeAlertContainer}>
            <Text style={styles.feeAlertDescription}>
            {I18n.t('components.organism.CheckoutSummary.fee_alert_description')}
            </Text>
            <TouchableWithoutFeedback onPress={this.handleOpenHelp}>
              <Text style={styles.feeAlertLearnMore}>{I18n.t('components.organism.CheckoutSummary.learn_more')}</Text>
            </TouchableWithoutFeedback>
          </View>
        )}
        <Dash dashColor={borderColor} dashGap={2} dashLength={8} dashThickness={1} />
        <View style={styles.totalWrapper}>
          <Text style={styles.totalLabel}>{I18n.t('components.organism.CheckoutSummary.total')}</Text>
          <Text style={styles.totalValue}>
            {totalPrice ? (
              localLabelFromBCH(totalPrice, paymentMethod)
            ) : (
              I18n.t('components.organism.CheckoutSummary.calculating')
            )}
          </Text>
        </View>
        <CouponApplyModal
          listingDetail={listing.listing}
          visible={showCouponModal}
          onSuccess={this.handleCouponSelect}
          hideModal={this.hideCouponModal}
        />
      </InputGroup>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
  checkoutNote: state.appstate.checkoutNote,
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(withNavigation(CheckoutSummary));
