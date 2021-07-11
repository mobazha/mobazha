import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, Image, Alert, ScrollView } from 'react-native';
import { NavigationActions } from 'react-navigation';
import { get, isEqual, isEmpty, capitalize, fromPairs } from 'lodash';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import InputGroup from '../components/atoms/InputGroup';
import RadioGroup from '../components/atoms/RadioGroup';
import NavBackButton from '../components/atoms/NavBackButton';
import BuyWyre from '../components/molecules/BuyWyre';
import { primaryTextColor, secondaryTextColor, brandColor, staticLabelColor } from '../components/commonColors';

import { setPaymentMethod } from '../reducers/appstate';
import { fetchWalletBalance, getFees } from '../reducers/wallet';

import { getRenderingCoins } from '../utils/coins';
import { screenWrapper } from '../utils/styles';
import { convertorsMap } from '../selectors/currency';
import { eventTracker } from '../utils/EventTracker';
import { DEFAULT_FEE_LEVELS, getFeeLevelDescription } from '../utils/fee';
import { minUnitAmountToBCH } from '../utils/currency';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    flex: 1,
    justifyContent: 'space-between',
  },
  feeLevelWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    flex: 1,
  },
  leftWrapper: {
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    paddingLeft: 8,
  },
  coinImage: {
    width: 40,
    height: 40,
    marginRight: 4,
  },
  label: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  secondary: {
    fontSize: 12,
    letterSpacing: 0,
    marginTop: 2,
    color: staticLabelColor,
  },
  description: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: secondaryTextColor,
    marginLeft: 5,
  },
  content: {
    flex: 1,
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  disabled: {
    opacity: 0.3,
  },
  leftPart: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'flex-start',
  },
  explanation: {
    textAlign: 'right',
    fontSize: 14,
    letterSpacing: 0,
    color: '#FF3B30',
  },
  rightWrapper: {
    alignItems: 'flex-end',
    justifyContent: 'space-between',
  },
  priceByLocalCurrency: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
};

const renderFeeLevel = fee => (
  <View style={styles.feeLevelWrapper}>
    <Text style={styles.label}>{fee.label}</Text>
    <Text style={styles.description}>{fee.description}</Text>
  </View>
);

const FEE_LEVEL_KEYS = ['superEconomic', 'economic', 'normal', 'priority'];

class PaymentMethod extends PureComponent {
  constructor(props) {
    super(props);
    const { paymentMethod, feeLevel } = props;
    this.state = {
      paymentMethod,
      feeLevel: 0,
      feeLevels: {},
    };
  }

  componentDidMount() {
    const { fetchWalletBalance, getFees } = this.props;
    fetchWalletBalance();
    getFees();
    this.updateFeeLevel({});
  }

  componentDidUpdate(prevProps) {
    this.updateFeeLevel(prevProps);
  }


  updateFeeLevel = (prevProps) => {
    const { fees: mainFees, feeLevel } = this.props;
    const { feeLevels } = this.state;
    if (!isEmpty(mainFees) && isEmpty(feeLevels)) {
      const methods = Object.keys(mainFees);
      const parsedList = methods.map(method => ([
        method,
        // We use FEE_LEVEL_KEYS instead of Object.keys(mainFees[method]) to stay in the fixed order
        FEE_LEVEL_KEYS.map(level => ({
          label: level === 'superEconomic' ? I18n.t('screens.paymentMethod.Super_Economic') : I18n.t('screens.paymentMethod.'+capitalize(level)),
          value: level,
          description: getFeeLevelDescription(level),
          fee: mainFees[method][level],
        })),
      ]));
      if (!isEmpty(feeLevel)) {
        const idx = FEE_LEVEL_KEYS.findIndex(level => feeLevel.value === level);
        this.setState({ feeLevel: idx });
      }
      this.setState({ feeLevels: fromPairs(parsedList) });
    }
    if (!isEqual(feeLevel, prevProps.feeLevel)) {
      const idx = FEE_LEVEL_KEYS.findIndex(level => level === feeLevel.value);
      this.setState({ feeLevel: idx });
    }
  }

  isCheckDisabled = () => {
    const { fees } = this.props;
    const { feeLevels } = this.state;
    return isEmpty(fees) || isEmpty(feeLevels);
  }

  handleFeeLevelChange = (feeLevel) => {
    this.setState({ feeLevel });
  }

  handleCoinChange = (idx) => {
    const renderingCoins = getRenderingCoins(this.props.navigation.getParam('acceptedCurrencies'));
    this.setState({ paymentMethod: renderingCoins[idx].value });
  }

  goBack = () => {
    this.props.navigation.dispatch(NavigationActions.back());
  }

  toNextScreen = () => {
    const { paymentMethod, feeLevel, feeLevels } = this.state;
    const feeLevelValue = get(feeLevels, `[${paymentMethod}][${feeLevel}]`);
    if (!isEmpty(feeLevelValue)) {
      eventTracker.trackEvent('Checkout-ChangedPaymentMethod', { paymentMethod });
      this.props.setPaymentMethod({
        paymentMethod,
        feeLevel: feeLevelValue,
      });
      this.props.navigation.goBack();
    } else {
      Alert.alert(I18n.t('screens.paymentMethod.select_fee_level'));
    }
  };

  renderPaymentMethod = (coin) => {
    const { localLabelFromLocal, convertBalanceFromBCH } = this.props;
    const { value: coinName } = coin;
    const balance = !coin.disabled && this.props.balance[coinName];
    const { cBalance } = convertBalanceFromBCH(balance, coinName);
    return (
      <View style={styles.wrapper}>
        <View style={styles.content}>
          <View style={[styles.leftPart, coin.disabled && styles.disabled]}>
            <Image source={coin.icon} style={styles.coinImage} />
            <View style={styles.leftWrapper}>
              <Text style={styles.label}>{coin.label}</Text>
              <Text style={styles.secondary}>{coinName}</Text>
            </View>
          </View>
          <View style={{ flex: 1 }} />
          {coin.disabled ? (
            <Text style={styles.explanation}>{I18n.t('screens.paymentMethod.not_accepted')}</Text>
          ) : (
            <View style={styles.rightWrapper}>
              <Text style={styles.priceByLocalCurrency}>
                {localLabelFromLocal(cBalance)}
              </Text>
              <Text style={styles.secondary}>
                {(coin.disabled || !balance) ? I18n.t('screens.paymentMethod.coming_soon') : `${minUnitAmountToBCH(balance.confirmed, coinName)} ${coinName}`}
              </Text>
            </View>
          )}
        </View>
      </View>
    );
  }

  render() {
    const { paymentMethod, feeLevel, feeLevels } = this.state;
    const chosenLevels = feeLevels[paymentMethod] || DEFAULT_FEE_LEVELS;
    const renderingCoins = getRenderingCoins(this.props.navigation.getParam('acceptedCurrencies'));
    const methodIdx = renderingCoins.findIndex(c => c.value === paymentMethod);
    const isDisabled = this.isCheckDisabled();
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.goBack}
          title={I18n.t('screens.paymentMethod.payment_method')}
          right={<LinkText color={isDisabled ? staticLabelColor : brandColor} text={I18n.t('screens.paymentMethod.done')} />}
          rightDisabled={isDisabled}
          onRight={this.toNextScreen}
        />
        <ScrollView>
          <InputGroup title={I18n.t('screens.paymentMethod.payment_method')} >
            <RadioGroup
              options={renderingCoins}
              selected={methodIdx}
              onChange={this.handleCoinChange}
              renderItem={this.renderPaymentMethod}
            />
          </InputGroup>
          <InputGroup title={I18n.t('screens.paymentMethod.transaction_speed')}>
            <RadioGroup
              options={chosenLevels}
              selected={feeLevel}
              onChange={this.handleFeeLevelChange}
              renderItem={renderFeeLevel}
            />
          </InputGroup>
          <BuyWyre />
        </ScrollView>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  paymentMethod: state.appstate.paymentMethod,
  feeLevel: state.appstate.feeLevel,
  balance: state.wallet.balance,
  fees: state.wallet.fees,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  setPaymentMethod,
  fetchWalletBalance,
  getFees,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(PaymentMethod);
