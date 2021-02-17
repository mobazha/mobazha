import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert } from 'react-native';
import { NavigationEvents } from 'react-navigation';
import Toast from 'react-native-root-toast';

import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import PurchaseStateTemplate from '../components/templates/PurchaseState';
import { purchaseListing } from '../reducers/order';
import { getGeneralCoinInfo, minUnitAmountToBCH } from '../utils/currency';
import { convertorsMap } from '../selectors/currency';

import { screenWrapper } from '../utils/styles';
import OBSlidingPanel from '../components/atoms/OBSlidingPanel';
import PayPanel from '../components/organism/PayPanel';
import { TIP_ADDRESSES } from '../utils/coins';
import { TOAST_OPTION } from '../utils/toast';

import {I18n} from '../langs/I18n';
class PurchaseState extends PureComponent {
  state = {
    paymentState: 'pending',
    percentage: 0,
    paidOrderInfo: null,
  };

  componentDidMount() {
    this.doPurchase();
  }

  onFailure = (reason) => {
    this.setState({ paymentState: 'error', reason });
  }

  onHalf = (orderInfo) => {
    const { navigation } = this.props;
    const orderData = navigation.getParam('orderData');
    const walletType = navigation.getParam('walletType');
    const { amount, paymentAddress } = orderInfo;
    if (walletType === 'external') {
      const screenKey = navigation.getParam('screenKey');
      this.setState({ percentage: 100, paymentState: 'success' }, () => {
        navigation.replace({
          routeName: 'ExternalPay',
          params: {
            amount: amount.amount,
            paymentAddress,
            coin: orderData.paymentCoin,
            orderId: orderInfo.orderId,
            screenKey,
          },
        });
      });
    } else {
      this.setState({ percentage: 50 });
    }
  }

  finishLoading = () => {
    this.setState({ paymentState: 'success' });
  }

  doPurchase = () => {
    const { navigation, purchaseListing } = this.props;
    const orderData = navigation.getParam('orderData');
    const walletType = navigation.getParam('walletType');
    const peerID = navigation.getParam('peerID');

    this.setState({ paymentState: 'pending' });

    purchaseListing({
      orderData,
      peerID,
      walletType,
      onSuccess: this.handleSuccess,
      onFailure: this.onFailure,
      onHalf: this.onHalf,
    });
  }

  handleGoBack = () => {
    const { paymentState } = this.state;
    const { navigation } = this.props;
    const screenKey = navigation.getParam('screenKey');
    if (paymentState === 'success') {
      this.props.navigation.goBack(screenKey);
    } else {
      this.props.navigation.goBack(null);
    }
  }

  handleShowPanel = (amount) => {
    this.slidingPanel.show();
    this.setState({ tipAmount: amount });
  }

  handleExternalTipPay = () => {
    this.isTipPay = true;
    this.slidingPanel.hide();

    const { navigation, paymentMethod } = this.props;
    const { tipAmount } = this.state;
    navigation.navigate({
      routeName: 'ExternalPay',
      params: {
        amount: tipAmount,
        paymentAddress: TIP_ADDRESSES[paymentMethod],
        coin: paymentMethod,
      },
    });
  }

  handleInternalTipPay = () => {
    this.isTipPay = true;
    this.slidingPanel.hide();

    const { navigation, paymentMethod } = this.props;
    const { tipAmount } = this.state;
    const screenKey = navigation.getParam('screenKey');
    navigation.navigate({
      routeName: 'SendMoney',
      params: {
        coin: paymentMethod,
        amount: Math.ceil(tipAmount),
        address: TIP_ADDRESSES[paymentMethod],
        screenKey,
      },
    });
  }

  setSlidingPanel = (ref) => { this.slidingPanel = ref; }

  handleSuccess = (paidOrderInfo) => {
    this.setState({ percentage: 100, paymentState: 'success', paidOrderInfo });
  }

  handleFocus = () => {
    if (this.hasFocusedOnce && this.isTipPay) {
      Toast.show('Thank you!', TOAST_OPTION);
    }
    this.hasFocusedOnce = true;
  }

  render() {
    const { navigation, paymentMethod } = this.props;
    const { paidOrderInfo } = this.state;

    const orderData = navigation.getParam('orderData');
    const price = navigation.getParam('price');

    const {
      paymentState, reason, percentage, tipAmount,
    } = this.state;
    const { localCurrency, localLabelFromBCH } = this.props;
    const { paymentCoin } = orderData;
    const coinInfo = getGeneralCoinInfo(paymentCoin);
    const dispAmount = localLabelFromBCH(price, paymentCoin);
    return (
      <View style={screenWrapper.wrapper}>
        <NavigationEvents onDidFocus={this.handleFocus} />
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <PurchaseStateTemplate
          paymentState={paymentState}
          reason={reason}
          localCurrency={localCurrency}
          coinInfo={coinInfo}
          dispAmount={dispAmount}
          onRetry={this.doPurchase}
          navigation={navigation}
          percentage={percentage}
          onFinish={this.finishLoading}
          paidOrderInfo={paidOrderInfo}
          onAmountSelected={this.handleShowPanel}
        />
        <OBSlidingPanel ref={this.setSlidingPanel}>
          <PayPanel
            amount={tipAmount}
            paymentMethod={paymentMethod}
            onInternal={this.handleInternalTipPay}
            onExternal={this.handleExternalTipPay}
          />
        </OBSlidingPanel>

      </View>
    );
  }
}

const mapStateToProps = state => ({
  paymentMethod: state.appstate.paymentMethod,
  ...convertorsMap(state),
});

const mapDispatchToProps = { purchaseListing };

export default connect(mapStateToProps, mapDispatchToProps)(PurchaseState);
