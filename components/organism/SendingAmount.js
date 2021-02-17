import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback, Image, StyleSheet } from 'react-native';
import * as _ from 'lodash';

import { convertorsMap } from '../../selectors/currency';
import { minUnitAmountToBCH, getPriceInMinimumUnit, parseCurrencyStr, getFixedCurrency, getDecimalPoints } from '../../utils/currency';
import SelectorModal from '../organism/SelectorModal';
import VirtualKeyboard from './VirtualKeyboard';
import { COINS } from '../../utils/coins';
import { borderColor } from '../commonColors';
import FEE_PLANS from '../../config/feePlans';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    alignItems: 'center',
    backgroundColor: 'transparent',
  },
  actionButton: {
    borderWidth: StyleSheet.hairlineWidth,
    borderColor,
    borderRadius: 2,
    height: 33,
    fontSize: 15,
    lineHeight: 15,
    color: 'white',
    textAlign: 'center',
    fontWeight: '600',
  },
  switch: {
    width: 48,
    marginHorizontal: 12,
    paddingHorizontal: 8,
    paddingTop: 10,
    paddingBottom: 8,
    textAlignVertical: 'center',
  },
  sendAll: {
    paddingHorizontal: 8,
    paddingTop: 10,
    paddingBottom: 8,
  },
  actionLeft: {
    width: 88,
  },
  actionRight: {
    width: 88,
    alignItems: 'flex-start',
  },
  feeWrapper: {
    alignSelf: 'flex-end',
  },
  feeInput: {
    textDecorationLine: 'none',
    marginTop: 10,
    marginBottom: 8,
    marginRight: 8,
    marginLeft: 8,
    fontWeight: '600',
  },
  amountInput: {
    width: '100%',
    color: 'white',
    fontSize: 44,
    lineHeight: 44,
    fontWeight: 'bold',
    paddingVertical: 0,
    paddingHorizontal: 20,
    textAlign: 'center',
  },
  localSymbol: {
    color: 'white',
    fontSize: 32,
    fontWeight: 'normal',
  },
  cryptoSymbol: {
    color: 'white',
    fontSize: 24,
    fontWeight: 'normal',
  },
  counterAmount: {
    color: 'white',
    fontSize: 18,
    marginBottom: 4,
  },
  icon: {
    marginBottom: 16,
    width: 40,
    height: 40,
  },
  actions: {
    flexDirection: 'row',
    alignSelf: 'center',
    marginTop: 16,
  },
};

class SendingAmount extends PureComponent {
  // eslint-disable-next-line react/sort-comp
  keyboardRef = React.createRef();

  state = {
    dispAmount: '0',
    isCrypto: false,
    sendAll: false,
  };

  componentDidMount() {
    const { amount, baseCoin, convertCurrencyFromBCH, setPressMaxHandler, onChange, localCurrency } = this.props;
    setPressMaxHandler(this.handlePressMax);

    if (amount) {
      const decimalPoints = getDecimalPoints(localCurrency);
      const dispAmount = convertCurrencyFromBCH(
        parseFloat(amount),
        baseCoin,
      );

      // This could have been this.setState({ dispAmount: `${dispAmount}` }) but we are doing this to cut to the limit
      this.setState({ dispAmount: `${parseFloat(getFixedCurrency(dispAmount, decimalPoints))}` }, () => {
        onChange(amount);
      });
    }
  }

  componentDidUpdate(prevProps) {
    const { baseCoin, amount, onChange } = this.props;
    if ((amount !== prevProps.amount && amount === 0) || (baseCoin !== prevProps.baseCoin)) {
      this.setState({ dispAmount: '0' });
      onChange(0);
    }
  }

  getFeeTitleLabel = option => option.displayLabel;
  getFeeLabel = option => option.label;

  getCryptoAmountFromFiat = (value) => {
    const floatValue = parseCurrencyStr(value);
    const { baseCoin, convertCurrencyToBCH } = this.props;
    return Math.floor(convertCurrencyToBCH(floatValue, baseCoin));
  }

  getFiatAmountFromCrypto = (value) => {
    const floatValue = parseCurrencyStr(value);
    const { baseCoin, convertCurrencyFromBCH } = this.props;
    return convertCurrencyFromBCH(
      parseFloat(getPriceInMinimumUnit(floatValue, baseCoin)),
      baseCoin,
    );
  }

  handlePressMax = () => {
    const { baseCoin, maxAmount, convertCurrencyFromBCH, onChange, onSendAll } = this.props;
    const { isCrypto } = this.state;
    const dispAmount = isCrypto ? (
      minUnitAmountToBCH(maxAmount, baseCoin, false)
    ) : (
      convertCurrencyFromBCH(maxAmount, baseCoin)
    );

    this.setState({ dispAmount: `${dispAmount}`, sendAll: true }, () => {
      this.keyboardRef.current.setText(this.getClippedAmountStr());

      onChange(maxAmount);
      onSendAll(true);
    });
  }

  handleSwitchCurrency = () => {
    const { isCrypto } = this.state;
    this.setState({ isCrypto: !isCrypto, dispAmount: this.getClippedCounterAmountStr() }, () => {
      this.keyboardRef.current.setText(this.getClippedAmountStr());
    });
  }

  handleChange = (value) => {
    const floatValue = parseCurrencyStr(value);
    const { baseCoin, convertCurrencyToBCH, onChange, onSendAll } = this.props;
    const { isCrypto } = this.state;

    const cryptoAmount = isCrypto ? (
      parseFloat(getPriceInMinimumUnit(floatValue, baseCoin))
    ) : (
      Math.floor(convertCurrencyToBCH(floatValue, baseCoin))
    );
    this.setState({ dispAmount: value, sendAll: false }, () => {
      onChange(cryptoAmount);
      onSendAll(false);
    });
  }

  getClippedAmountStr = (leaveTrailingZero) => {
    const { localCurrency, baseCoin } = this.props;
    const { dispAmount, isCrypto } = this.state;

    if (leaveTrailingZero) {
      return dispAmount;
    } else {
      const decimalPoints = getDecimalPoints(isCrypto ? baseCoin : localCurrency);
      const floatValue = parseCurrencyStr(dispAmount);
      return `${parseFloat(getFixedCurrency(floatValue, decimalPoints))}`;
    }
  }

  getClippedCounterAmountStr = () => {
    const { baseCoin, localCurrency } = this.props;
    const { dispAmount, isCrypto } = this.state;
    const counterAmount = isCrypto ? (
      this.getFiatAmountFromCrypto(dispAmount)
    ) : (
      minUnitAmountToBCH(this.getCryptoAmountFromFiat(dispAmount), baseCoin)
    );
    const decimalPoints = getDecimalPoints(isCrypto ? localCurrency : baseCoin);
    const parsedAmount = parseFloat(getFixedCurrency(counterAmount, decimalPoints));
    return `${parsedAmount}`;
  }

  renderFee = () => {
    const { feeLevel, onFeeChange } = this.props;
    return (
      <SelectorModal
        white
        compact
        options={FEE_PLANS}
        value={feeLevel}
        onChange={onFeeChange}
        getLabel={this.getFeeLabel}
        getTitleLabel={this.getFeeTitleLabel}
        noBorder
        wrapperStyle={[styles.actionButton, styles.feeWrapper]}
        inputStyle={styles.feeInput}
      />
    );
  }

  render() {
    const { baseCoin, localCurrency, localSymbol } = this.props;
    const { isCrypto } = this.state;
    const decimalPoints = getDecimalPoints(isCrypto ? baseCoin : localCurrency);
    return (
      <View style={styles.wrapper}>
        <View style={{ flex: 1 }} />
        <Image style={styles.icon} source={COINS[baseCoin].icon} resizeMode="cover" />
        <Text style={styles.amountInput} numberOfLines={1}>
          {!isCrypto && <Text style={styles.localSymbol}>{localSymbol}</Text>}
          {this.getClippedAmountStr(true)}
          {isCrypto && <Text style={styles.cryptoSymbol}>{` ${baseCoin}`}</Text>}
        </Text>
        <Text style={styles.counterAmount}>
          {isCrypto && <Text>{localSymbol}</Text>}
          {this.getClippedCounterAmountStr()}
          {!isCrypto && <Text>{` ${baseCoin}`}</Text>}
        </Text>
        <View style={styles.actions}>
          <View style={styles.actionLeft}>
            {this.renderFee()}
          </View>
          <TouchableWithoutFeedback onPress={this.handleSwitchCurrency}>
            <Text style={[styles.actionButton, styles.switch]}>
              {isCrypto ? baseCoin : localCurrency}
            </Text>
          </TouchableWithoutFeedback>
          <View style={styles.actionRight}>
            <TouchableWithoutFeedback onPress={this.handlePressMax}>
              <Text style={[styles.actionButton, styles.sendAll]}>
              {I18n.t('components.organism.SendingAmount.send_all')}
              </Text>
            </TouchableWithoutFeedback>
          </View>
        </View>
        <View style={{ flex: 1 }} />
        <VirtualKeyboard
          ref={this.keyboardRef}
          color="white"
          pressMode="string"
          onPress={this.handleChange}
          decimal
          decimalPoints={decimalPoints}
        />
        <View style={{ flex: 1 }} />
      </View>
    );
  }
}

export default connect(convertorsMap, null, null, { withRef: true })(SendingAmount);
