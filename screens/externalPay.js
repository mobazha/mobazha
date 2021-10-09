import React, { PureComponent } from 'react';
import { connect } from 'react-redux';

import { Image, View, Clipboard, Text, TouchableOpacity } from 'react-native';
import QRCode from 'react-native-qrcode-svg';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import Header from '../components/molecules/Header';
import { screenWrapper } from '../utils/styles';
import { COINS } from '../utils/coins';
import { minUnitAmountToBCH } from '../utils/currency';
import { getCoinUpdateAddress } from '../api/wallet';

import { primaryTextColor, borderColor, formLabelColor } from '../components/commonColors';
import NavBackButton from '../components/atoms/NavBackButton';

import { simulatePayment } from '../reducers/notifications';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    alignItems: 'center',
  },
  title: {
    fontSize: 15,
    marginVertical: 30,
    color: formLabelColor,
  },
  cryptoWrapper: {
    marginBottom: 20,
    flexDirection: 'row',
    alignItems: 'center',
  },
  crypto: {
    fontSize: 17,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  coinAddress: {
    marginTop: 20,
    textAlign: 'center',
    paddingHorizontal: 20,
    fontSize: 13,
    fontWeight: 'normal',
    color: primaryTextColor,
  },
  doCopy: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    marginLeft: 6,
  },
  icon: {
    width: 32,
    height: 32,
  },
  button: {
    width: '100%',
    height: ifIphoneX(88, 54),
    paddingBottom: ifIphoneX(34, 0),
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    borderTopWidth: 1,
    borderColor,
  },
  qrcodeWrapper: {
    alignItems: 'center',
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 180,
    top: 300,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

class ExternalPay extends PureComponent {
  // eslint-disable-next-line react/sort-comp
  constructor(props) {
    super(props);

    this.state = {
      copied: null,
      paymentAddress: "",
    };

    this.updatePaymentAddress();
  }

  updatePaymentAddress() {
    const { navigation } = this.props;
    const coin = navigation.getParam('coin');
    if (coin != 'CFX') {
      this.setState({ paymentAddress: navigation.getParam('paymentAddress')});
    } else {
      getCoinUpdateAddress(coin, navigation.getParam('paymentAddress')).then(response => {
        this.setState({ paymentAddress: response.address});
      });
    }
  }

  componentDidUpdate(prevProps) {
    const { payment, navigation } = this.props;
    const orderId = navigation.getParam('orderId');
    if (payment.orderId === orderId && prevProps.payment.orderId !== orderId) {
      const screenKey = navigation.getParam('screenKey');
      navigation.replace('ExternalPaySuccess', { orderId, screenKey });
    }
  }

  handleCopyLink = () => {
    const { navigation } = this.props;
    const SIMULATE_SOCKET_MESSAGE = false;
    if (SIMULATE_SOCKET_MESSAGE) {
      const { simulatePayment } = this.props;
      simulatePayment({
        coinType: 'BCH',
        fundingTotal: 836190,
        notificationId: 'QmUPHyoFP56eouMGcjBAdSBoiBcCY3A7DhZMfGmKKwTsBe',
        orderId: navigation.getParam('orderId'),
        type: 'payment',
      });
    } else {
      const { paymentAddress } = this.state;
      Clipboard.setString(paymentAddress);

      this.setState({ copied: I18n.t('screens.externalPay.address_copied')});
      setTimeout(() => this.setState({ copied: null }), 2000);
    }
  };

  handleCopyAmount = () => {
    const { navigation } = this.props;
    const amount = navigation.getParam('amount');
    const coin = navigation.getParam('coin');
    Clipboard.setString(`${minUnitAmountToBCH(amount, coin)}`);

    this.setState({ copied: I18n.t('screens.externalPay.amount_copied') });
    setTimeout(() => this.setState({ copied: null }), 2000);
  };

  handleGoBack = () => {
    const { navigation } = this.props;
    const screenKey = navigation.getParam('screenKey');
    navigation.goBack(screenKey);
  }

  render() {
    const { navigation } = this.props;
    const amount = navigation.getParam('amount');
    const coin = navigation.getParam('coin');
    const { icon, qrLabel } = COINS[coin];

    const { copied, paymentAddress } = this.state;

    return (
      <View style={[screenWrapper.wrapper, styles.wrapper]}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <Text style={styles.title}> {I18n.t('screens.externalPay.pay_order')}</Text>
        <TouchableOpacity onPress={this.handleCopyAmount}>
          <View style={styles.cryptoWrapper}>
            <Image style={styles.icon} source={icon} resizeMode="cover" />
            <Text style={styles.crypto}>
              {` ${minUnitAmountToBCH(amount, coin)} ${coin}`}
            </Text>
          </View>
        </TouchableOpacity>
        <TouchableOpacity onPress={this.handleCopyLink}>
          <View style={styles.qrcodeWrapper}>
            <QRCode value={`${qrLabel}:${paymentAddress}?amount=${minUnitAmountToBCH(amount, coin)}`} size={200} />
            <Text style={styles.coinAddress}>{paymentAddress}</Text>
          </View>
        </TouchableOpacity>
        <View style={{ flex: 1 }} />
        <TouchableOpacity
          style={styles.button}
          onPress={this.handleCopyLink}
        >
          <Ionicons
            name="md-copy"
            color={primaryTextColor}
            size={24}
          />
          <Text style={styles.doCopy}>{I18n.t('screens.externalPay.copy_address')}</Text>
        </TouchableOpacity>
        {copied && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{copied}</Text>
          </View>
        )}
      </View>
    );
  }
}
const mapStateToProps = state => ({
  payment: state.notifications.payment,
});

const mapDispatchToProps = {
  simulatePayment,
};

export default connect(mapStateToProps, mapDispatchToProps)(ExternalPay);
