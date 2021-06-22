import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert } from 'react-native';

import TabHeader from '../components/organism/TabHeader';
import SendMoneyTemplate from '../components/templates/SendMoney';
import NavBackButton from '../components/atoms/NavBackButton';

import { fetchWalletBalance, sendFunds, resetStatus, getFees } from '../reducers/wallet';
import { fetchExchangeRate } from '../reducers/exchangeRate';
import { convertorsMap } from '../selectors/currency';
import { screenWrapper } from '../utils/styles';
import { isCrypto, getBigCurrencyInfo } from '../utils/currency';
import { estimateFee, getWalletFailureMessage } from '../api/wallet';

import {I18n} from '../langs/I18n';

class SendMoney extends PureComponent {
  constructor(props) {
    super(props);
    const coin = props.navigation.getParam('coin') || props.coin;
    const address = props.navigation.getParam('address') || '';
    const amount = props.navigation.getParam('amount') || '';
    this.state = {
      coin,
      address,
      amount: amount || 0,
      memo: '',
      feeLevel: 'SUPER_ECONOMIC',
      sendingAll: false,
      estimating: false,
    };
  }

  componentDidMount() {
    const { fetchWalletBalance, fetchExchangeRate, getFees } = this.props;
    const { coin } = this.state;
    fetchWalletBalance();
    fetchExchangeRate(coin);
    getFees();
  }

  onChangeCofinInfo = (info) => {
    const { coin } = this.state;
    if (coin !== info.coin) {
      this.props.fetchExchangeRate(info.coin);
    }
    this.setState({ ...info });
  }

  confirmPayment = async () => {
    const { amount, feeLevel, coin, sendingAll } = this.state;
    const {
      convertCurrency, localSymbol, localCurrency, fees, convertCurrencyFromBCH,
    } = this.props;
    const fixedCount = isCrypto(localCurrency) ? 8 : 2;

    let estimatedFee;
    if (sendingAll) {
      const coinFee = fees[coin];
      estimatedFee = coinFee ? (coinFee[feeLevel.toLowerCase()].amount || 0) * 400 : 0;
    } else {
      this.setState({ estimating: true });
      const estimateResponse = await estimateFee(coin, amount, feeLevel);
      this.setState({ estimating: false });
      if (estimateResponse.success === false) {
        const { reason } = estimateResponse;
        Alert.alert(getWalletFailureMessage(reason));
        return;
      }
      estimatedFee = estimateResponse.amount;
    }

    const sendingAmount = convertCurrency(amount, coin);
    const includedFee = convertCurrencyFromBCH(estimatedFee || 0, coin);
    if (sendingAll) {
      const sendingLabel = `${localSymbol}${sendingAmount.toFixed(fixedCount)}`;
      const deductingLabel = `${localSymbol}${includedFee.toFixed(fixedCount)}`;
      const recipientLabel = `${localSymbol}${sendingAmount.minus(includedFee).toFixed(fixedCount)}`;
      Alert.alert(
        `Send ${sendingLabel}?`,
        `${deductingLabel} network fee will be deducted. The recipient will receive ${recipientLabel}.`,
        [
          { text: 'Cancel' },
          { text: 'Send', onPress: this.sendMoney },
        ],
      );
    } else {
      const sendingLabel = `${localSymbol}${sendingAmount.plus(includedFee).toFixed(fixedCount)}`;
      const includingLabel = `${localSymbol}${includedFee.toFixed(fixedCount)}`;
      const recipientLabel = `${localSymbol}${sendingAmount.toFixed(fixedCount)}`;

      Alert.alert(
        `Send ${sendingLabel}?`,
        `${includingLabel} fee will be included. The recipient will receive ${recipientLabel}.`,
        [
          { text: 'Cancel' },
          { text: 'Send', onPress: this.sendMoney },
        ],
      );
    }
  }

  sendMoney = () => {
    const {
      address, memo, feeLevel, amount, coin, sendingAll,
    } = this.state;
    const { navigation, sendFunds } = this.props;

    sendFunds({
      data: {
        amount: amount.toString(),
        currency: getBigCurrencyInfo(coin),
        currencyCode: coin,
        address: (address.includes(':') && !address.includes('cfx')) ? address.split(':')[1] : address,
        feeLevel,
        memo,
        requireAssociatedOrder: false,
        spendAll: sendingAll,
      },
    });
    navigation.navigate({
      routeName: 'PaymentSuccess',
      params: {
        amount,
        wallet: coin,
        feeLevel,
        memo,
        address,
        onSuccess: this.handlePaymentSuccess,
      },
    });
  }

  handlePaymentSuccess = () => {
    this.setState({
      amount: 0,
      address: '',
      feeLevel: 'SUPER_ECONOMIC',
    });
  }

  handleGoBack = () => this.props.navigation.goBack()

  handleFeeChange = (val) => {
    this.setState({ feeLevel: val.value });
  }

  handleChangeSendingAll = (sendingAll) => {
    this.setState({ sendingAll });
  }

  resetStatus = () => this.props.resetStatus()

  handleSetPressMaxHandler = (handler) => {
    this.handlePressMax = handler;
  };

  render() {
    const {
      coin, address, amount, feeLevel, estimating,
    } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <TabHeader left={<NavBackButton white />} onLeft={this.handleGoBack} />
        <SendMoneyTemplate
          amount={amount}
          coin={coin}
          address={address}
          onChange={this.onChangeCofinInfo}
          onSend={this.confirmPayment}
          setPressMaxHandler={this.handleSetPressMaxHandler}
          feeLevel={feeLevel}
          onFeeChange={this.handleFeeChange}
          onSendAll={this.handleChangeSendingAll}
          estimating={estimating}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  balance: state.wallet.balance,
  coin: state.appstate.sendMoneyCoin,
  reason: state.wallet.reason,
  fees: state.wallet.fees,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  fetchWalletBalance,
  fetchExchangeRate,
  sendFunds,
  resetStatus,
  getFees,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(SendMoney);
