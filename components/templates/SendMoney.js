import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { KeyboardAvoidingView, View, TouchableOpacity, Text, TouchableWithoutFeedback, ActivityIndicator } from 'react-native';
import * as _ from 'lodash';

import SendingAddressSelector from '../organism/SendingAddressSelector';
import SendingAmount from '../organism/SendingAmount';
import CoinTypeSelector from '../organism/CoinTypeSelector';
import { brandColor } from '../commonColors';

import { convertorsMap } from '../../selectors/currency';
import { setSendMoneyCoin } from '../../reducers/appstate';
import { OBLightModal } from './OBModal';
import { keyboardAvoidingViewSharedProps } from '../../utils/keyboard';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: brandColor,
    flex: 1,
  },
  sendFull: {
    fontWeight: 'bold',
    color: 'white',
    textDecorationLine: 'underline',
  },
  nextButton: {
    backgroundColor: 'white',
    paddingVertical: 16,
    marginHorizontal: 16,
    marginBottom: 16,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: 4,
  },
  nextButtonText: {
    fontSize: 16,
    fontWeight: 'bold',
    color: brandColor,
    lineHeight: 18,
  },
  disabled: {
    backgroundColor: 'rgba(255, 255, 255, 0.65)',
  },
  sendingAddress: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.65)',
  },
};

class SendMoney extends PureComponent {
  static getDerivedStateFromProps(props) {
    const {
      coin, address, amount,
    } = props;
    return ({
      coin, address, amount,
    });
  }

  state = {
    coin: '',
    address: '',
    amount: 0,
    showModal: false,
  };

  handleAddressChange = (address) => {
    const { onChange } = this.props;
    onChange({ ...this.state, address });
  }

  handleAmountChange = (amount) => {
    const { onChange } = this.props;
    onChange({ ...this.state, amount });
  }

  handleCoinChanged = (coin) => {
    const { onChange } = this.props;
    onChange({ ...this.state, coin });
  }

  handleShowModal = () => {
    this.setState({ showModal: true });
  }

  handleHideModal = () => {
    this.setState({ showModal: false });
  }

  handleSend = () => {
    const { onSend } = this.props;
    this.handleHideModal();
    setTimeout(onSend, 500);
  }

  resetAmount() {
    this.amountRef.resetAmount();
  }

  render() {
    const { balance, setPressMaxHandler, feeLevel, estimating, onFeeChange, onSendAll } = this.props;
    const {
      address, coin, amount, showModal,
    } = this.state;
    return (
      <View style={styles.wrapper}>
        <SendingAmount
          amount={amount}
          baseCoin={coin}
          maxAmount={balance[coin].confirmed}
          onChange={this.handleAmountChange}
          setPressMaxHandler={setPressMaxHandler}
          feeLevel={feeLevel}
          onFeeChange={onFeeChange}
          onSendAll={onSendAll}
        />
        <TouchableOpacity activeOpacity={1} onPress={this.handleShowModal} disabled={amount === 0 || estimating}>
          <View style={[styles.nextButton, amount === 0 || estimating ? styles.disabled : {}]}>
            {estimating ? (
              <ActivityIndicator size="small" color="white" />
            ) : (
              <Text style={styles.nextButtonText}>{I18n.t('components.templates.SendMoney.NEXT')}</Text>
            )}
          </View>
        </TouchableOpacity>
        <CoinTypeSelector coin={coin} onChange={this.handleCoinChanged} showBalance noBorder />
        <OBLightModal
          animationType="fade"
          overlay
          visible={showModal}
          onRequestClose={this.handleHideModal}
        >
          <KeyboardAvoidingView style={styles.sendingAddress} {...keyboardAvoidingViewSharedProps}>
            <TouchableWithoutFeedback onPress={this.handleHideModal}>
              <View style={{ flex: 1 }} />
            </TouchableWithoutFeedback>
            <SendingAddressSelector
              title= {I18n.t('components.templates.SendMoney.send_to')}
              address={address}
              onChange={this.handleAddressChange}
              placeholder={I18n.t('components.templates.SendMoney.paste_or_scan')}
              onSend={this.handleSend}
            />
          </KeyboardAvoidingView>
        </OBLightModal>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  balance: state.wallet.balance,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  setSendMoneyCoin,
};

export default connect(mapStateToProps, mapDispatchToProps, null, { withRef: true })(SendMoney);
