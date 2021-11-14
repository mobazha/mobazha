import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert, Text } from 'react-native';
import { NavigationActions } from 'react-navigation';
import { capitalize, isEmpty } from 'lodash';
import { ifIphoneX } from 'react-native-iphone-x-helper';
import BigNumber from 'bignumber.js';

import Header from '../components/molecules/Header';
import CheckoutTemplate from '../components/templates/Checkout';
import NavBackButton from '../components/atoms/NavBackButton';
import PayBanner from '../components/atoms/PayBanner';
import PayPanel from '../components/organism/PayPanel';

import { fetchWalletBalance } from '../reducers/wallet';
import { setPaymentMethod, setModerator, clearCheckoutNote, setShippingOption, clearShippingOption } from '../reducers/appstate';
import { fetchSettings } from '../reducers/settings';
import { fetchProfile } from '../reducers/profile';
import { fetchExchangeRate } from '../reducers/exchangeRate';
import { getProfile } from '../api/profile';
import { getFees } from '../api/wallet';
import { convertorsMap } from '../selectors/currency';
import { filterModerators } from '../selectors/moderators';
import { initShippingOptionWithMinShippingPrice } from '../utils/stockManage';
import { screenWrapper } from '../utils/styles';
import OBSlidingPanel from '../components/atoms/OBSlidingPanel';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

const FEE_MAX_PERCENT = 0.25;
const styles = {
  bottomWrapper: {
    paddingBottom: ifIphoneX(34, 0),
  },
};

class Checkout extends PureComponent {
  static getDerivedStateFromProps(props) {
    const {
      navigation, shippingAddress, shippingAddresses,
    } = props;
    const listing = navigation.getParam('listing');
    const { metadata: { contractType }, shippingOptions } = listing.listing;
    if (contractType !== 'PHYSICAL_GOOD' || !shippingOptions) {
      return {};
    }

    let country;
    if (!isEmpty(shippingAddresses)) {
      country = shippingAddresses[shippingAddress].country;
    }

    return {
      shippingOptions: isEmpty(shippingAddresses) ? (
        []
      ) : (
        shippingOptions && shippingOptions.filter(op => op.regions.includes('ALL') || op.regions.includes(country))
      ),
      contractType,
    };
  }

  constructor(props) {
    super(props);

    const {
      navigation, clearCheckoutNote, fetchExchangeRate,
    } = props;

    const quantity = navigation.getParam('quantity');

    this.state = {
      quantity,
      checkoutData: {},
      vendorName: '',
      moderators: [],
      loadingModerators: false,
      selectedCoupon: {},
    };
    fetchExchangeRate();
    clearCheckoutNote();

    this.initShippingOption = initShippingOptionWithMinShippingPrice.bind(this);
  }

  async componentDidMount() {
    const {
      fetchWalletBalance,
      fetchSettings,
      feeLevel = {},
      paymentMethod,
      navigation,
      setModerator,
      filterModerators,
    } = this.props;
    const listing = navigation.getParam('listing');

    fetchWalletBalance();
    fetchSettings();

    const {
      moderators,
      vendorID: { peerID },
    } = listing.listing || {};

    moderators.forEach((moderator) => {
      this.props.fetchProfile({ peerID: moderator, getLoaded: true });
    });

    // const filteredModerators = filterModerators(moderators);

    this.setState({ moderators: moderators });

    setModerator('');
    if (!isEmpty(moderators)) {
      this.setState({ loadingModerators: true });
      setTimeout(() => this.setModerator(), 2000);
    }

    try {
      const fees = await getFees();
      const feeResponse = fees[paymentMethod];
      const { value: feeLevelValue = 'superEconomic' } = feeLevel;
      this.handleSetPaymentMethod(feeResponse, feeLevelValue);
    } catch (err) {
      console.log('Failed to fetch fees', err);
    }

    try {
      const { name: vendorName } = await getProfile(peerID);
      this.setState({ vendorName });
    } catch (err) {
      console.log('Failed to fetch profile', err);
    }
  }

  componentDidUpdate(prevProps) {
    if (isEmpty(prevProps.shippingAddresses) && !isEmpty(this.props.shippingAddresses)) {
      this.initShippingOption();
    }
  }

  setModerator = () => {
    const { profiles, setModerator } = this.props;
    const { moderators } = this.state;
    if (!isEmpty(moderators)) {
      const nonEmptyModerators = moderators
        .map(moderator => profiles[moderator])
        .filter(profile => profile)
        .map(profile => profile.peerID);

      if (nonEmptyModerators.length > 0) {
        setModerator(nonEmptyModerators[Math.floor(Math.random() * nonEmptyModerators.length)]);
        this.setState({ loadingModerators: false });
        return;
      }
    }
    this.setState({ loadingModerators: false });
  }

  setSlidingPanel = (ref) => { this.slidingPanel = ref; }

  isOwnListing = () => {
    const { profile, navigation } = this.props;
    const listing = navigation.getParam('listing');
    const { peerID } = listing.listing.vendorID;
    return peerID === profile.peerID;
  }

  purchaseListing = (walletType) => {
    const { navigation } = this.props;
    const listing = navigation.getParam('listing');
    const { peerID } = listing.listing.vendorID;
    const { checkoutData, vendorName } = this.state;
    const {
      checkoutObject, price,
    } = checkoutData;

    const screenKey = navigation.getParam('screenKey');
    navigation.navigate({
      routeName: 'PurchaseState',
      params: {
        orderData: checkoutObject,
        price,
        vendorName,
        peerID,
        walletType,
        screenKey: screenKey || this.props.navigation.state.key,
      },
    });
  }

  handleSetPaymentMethod = (fees, val) => {
    this.props.setPaymentMethod({
      feeLevel: {
        label: val === 'superEconomic' ? I18n.t('screens.checkout.Super_Economic') : I18n.t('screens.checkout.'+capitalize(val)),
        value: val,
        fee: fees[val],
      },
    });
  }

  handleShowPanel = () => {
    if (this.isOwnListing()) {
      Alert.alert('Oops!', "You can't purchase your own listing.", [{ text: 'OK' }]);
    } else {
      const { price, fee = 0 } = this.state.checkoutData;
      if (price * FEE_MAX_PERCENT < fee) {
        Alert.alert('Oops', 'Fee is too high. Please use a different coin.', [{ text: 'OK' }]);
      } else {
        if (this.slidingPanel) {
          eventTracker.trackEvent('Checkout-Tapped-Pay');
          this.slidingPanel.show();
        }
      }
    }
  }

  handleCheckUnconfirmed = () => {
    const { balance } = this.props;
    const { checkoutObject, price } = this.state.checkoutData;
    const { confirmed } = balance[checkoutObject.paymentCoin];
    const confirmIsInsufficient = BigNumber(price).gt(BigNumber(confirmed));
    if (confirmIsInsufficient) {
      Alert.alert('Oops!', 'Item can be purchased after funds finishing transferring into your wallet.');
    } else {
      this.purchaseListing('internal');
    }
  }

  handlePay = walletType => () => {
    const { localLabelFromBCH } = this.props;
    const { checkoutObject, price } = this.state.checkoutData;
    if (!isEmpty(checkoutObject)) {
      if (walletType === 'internal') {
        eventTracker.trackEvent('Checkout-ChangeWallet', { walletType });
        Alert.alert(
          I18n.t('screens.checkout.pay_info', {amount:localLabelFromBCH(price, checkoutObject.paymentCoin)}), '',
          [
            { text: I18n.t('screens.checkout.cancel') },
            {
              text: I18n.t('screens.checkout.pay_now'),
              onPress: this.handleCheckUnconfirmed,
            },
          ],
        );
      } else {
        eventTracker.trackEvent('Checkout-ChangeWallet', { walletType });
        this.purchaseListing('external');
      }
    }
  }

  handleGoToShippingAddress = () => {
    const { navigation } = this.props;
    const listing = navigation.getParam('listing');
    const { shippingOptions, item } = listing.listing;
    const { code: currency } = item.priceCurrency;
    navigation.navigate({
      routeName: 'CheckoutShippingAddress',
      params: { shippingOptions, currency },
    });
  }

  handleGoToPaymentMethod = (acceptedCurrencies) => {
    eventTracker.trackEvent('Checkout-Tapped-PaymentMethod');
    this.props.navigation.navigate('PaymentMethod', { acceptedCurrencies });
  }

  handleGoToAddFund = (coin) => {
    this.props.navigation.navigate({
      routeName: 'CheckoutReceiveMoney',
      params: { coin },
    });
  }

  handleGoToModerators = () => {
    const { moderators } = this.state;
    this.props.navigation.navigate({
      routeName: 'CheckoutModerators',
      params: { moderators },
    });
  }

  handleSetCheckoutObject = (checkoutData) => {
    this.setState({ checkoutData });
  }

  isNotPayable = () => {
    const { shippingOptions, contractType } = this.state;
    if (contractType === 'PHYSICAL_GOOD') {
      if (!shippingOptions || shippingOptions.length === 0) {
        return true;
      } else {
        return false;
      }
    }
    return false;
  }

  handleChangeCoupon = (coupon) => {
    this.setState({ selectedCoupon: coupon });
  }

  render() {
    const {
      shippingAddress,
      shippingAddresses,
      shippingOption,
      balance,
      paymentMethod,
      feeLevel,
      localCurrency,
      navigation,
    } = this.props;

    const listing = navigation.getParam('listing');
    const combo = navigation.getParam('combo');

    const {
      quantity,
      selectedCoupon,
      loadingModerators,
      checkoutData,
    } = this.state;
    const { price } = checkoutData;

    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.dispatch(NavigationActions.back());
          }}
          title="Checkout"
        />
        <CheckoutTemplate
          listing={listing}
          combo={combo}
          quantity={quantity}
          shippingAddress={shippingAddresses[shippingAddress] || {}}
          shippingOption={shippingOption}
          balance={balance}
          paymentMethod={paymentMethod}
          feeLevel={feeLevel}
          loadingModerators={loadingModerators}
          toShippingAddress={this.handleGoToShippingAddress}
          toPaymentMethod={this.handleGoToPaymentMethod}
          toAddFund={this.handleGoToAddFund}
          toModerators={this.handleGoToModerators}
          setCheckoutObject={this.handleSetCheckoutObject}
          localCurrency={localCurrency}
          selectedCoupon={selectedCoupon}
          onChangeCoupon={this.handleChangeCoupon}
        />
        <View style={styles.bottomWrapper}>
          <PayBanner
            disabled={this.isNotPayable()}
            amount={price}
            paymentMethod={paymentMethod}
            onPay={this.handleShowPanel}
          />
        </View>
        <OBSlidingPanel
          ref={this.setSlidingPanel}
        >
          <PayPanel
            amount={price}
            paymentMethod={paymentMethod}
            onInternal={this.handlePay('internal')}
            onExternal={this.handlePay('external')}
          />
        </OBSlidingPanel>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  shippingAddresses: state.settings.shippingAddresses,
  shippingAddress: state.appstate.shippingAddress,
  shippingOption: state.appstate.shippingOption,
  paymentMethod: state.appstate.paymentMethod,
  moderator: state.appstate.moderator,
  feeLevel: state.appstate.feeLevel,
  balance: state.wallet.balance,
  profiles: state.profiles,
  profile: state.profile.data,
  ...convertorsMap(state),
  filterModerators: filterModerators(state),
});

const mapDispatchToProps = {
  fetchWalletBalance,
  fetchSettings,
  fetchProfile,
  fetchExchangeRate,
  setPaymentMethod,
  setModerator,
  clearCheckoutNote,
  setShippingOption,
  clearShippingOption,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Checkout);
