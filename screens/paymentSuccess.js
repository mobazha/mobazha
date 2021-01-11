import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Image, Text, TouchableWithoutFeedback, Animated, Keyboard, Linking } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';

import { sendFunds, resetStatus } from '../reducers/wallet';
import { getGeneralCoinInfo, getDecimalPoints, getBigCurrencyInfo } from '../utils/currency';
import { screenWrapper } from '../utils/styles';
import { transactionLinkDict } from '../utils/coins';
import { primaryTextColor, brandColor, borderColor, warningColor, formLabelColor, foregroundColor } from '../components/commonColors';
import { convertorsMap } from '../selectors/currency';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    flexDirection: 'column',
    justifyContent: 'center',
    alignItems: 'center',
    marginVertical: 25,
  },
  descriptionWrapper: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
  },
  coinImage: {
    width: 20,
    height: 20,
    marginRight: 6,
  },
  description: {
    fontSize: 16,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  buttonWrapper: {
    height: 56,
    borderTopWidth: 1,
    borderColor,
    justifyContent: 'center',
    alignItems: 'center',
  },
  buttonText: {
    fontSize: 13,
    fontWeight: 'bold',
    textAlign: 'center',
    color: primaryTextColor,
  },
  retryButtonText: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  retryButton: {
    marginTop: 12,
    width: 250,
    paddingVertical: 9,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
  },
  loadingWheel: {
    width: 70,
    height: 70,
    borderRadius: 35,
    borderTopWidth: 6,
    borderLeftWidth: 6,
    borderBottomWidth: 6,
    borderColor: primaryTextColor,
    marginBottom: 12,
  },
  iconWrapper: {
    width: 110,
    height: 110,
    justifyContent: 'center',
    alignItems: 'center',
  },
  statusTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  statusDescription: {
    marginTop: 16,
    fontSize: 15,
    fontWeight: 'normal',
    letterSpacing: 0,
    color: formLabelColor,
    width: 280,
    textAlign: 'center',
  },
  errorDescription: {
    fontSize: 13,
    color: formLabelColor,
    width: 280,
    marginBottom: 15,
  },
  errorTitle: {
    fontWeight: 'bold',
  },
};

class PaymentSuccess extends PureComponent {
  componentDidMount() {
    Keyboard.dismiss();
    this.animate();
  }

  componentDidUpdate(prevProps) {
    const { status: prevStatus } = prevProps;
    const { status, navigation } = this.props;
    const callback = navigation.getParam('onSuccess');
    if (prevStatus === 'sending' && status === 'send_succeed') {
      callback();
    }
  }

  aniVal = new Animated.Value(0);

  toTransactionDetails = () => {
    const { sentTxid, navigation } = this.props;
    const coin = navigation.getParam('wallet');
    Linking.openURL(transactionLinkDict(sentTxid)[coin]);
  }

  handleRetry = () => {
    const { navigation, sendFunds, resetStatus } = this.props;
    const address = navigation.getParam('address');
    const memo = navigation.getParam('memo');
    const feeLevel = navigation.getParam('feeLevel');
    const amount = navigation.getParam('amount');
    const wallet = navigation.getParam('wallet');
    resetStatus();
    this.animate();
    sendFunds({
      data: {
        amount: amount.toString(),
        currency: getBigCurrencyInfo(wallet),
        currencyCode: wallet,
        address: address.includes(':') ? address.split(':')[1] : address,
        feeLevel,
        memo,
        requireAssociatedOrder: false,
      },
    });
  }

  animate = () => {
    Animated.timing(this.aniVal, {
      duration: 1000,
      toValue: 1,
    }).start(() => {
      const { status } = this.props;
      this.aniVal.setValue(0);
      if (status === 'sending') {
        this.animate();
      }
    });
  }

  renderSuccess = () => {
    const { localLabelFromBCH, navigation } = this.props;
    const coin = navigation.getParam('wallet');
    const amount = navigation.getParam('amount');
    const coinInfo = getGeneralCoinInfo(coin);
    return (
      <React.Fragment>
        <View style={styles.wrapper}>
          <View style={styles.iconWrapper}>
            <Ionicons color={brandColor} size={120} name="md-checkmark" />
          </View>
          <View style={styles.descriptionWrapper}>
            <Image style={styles.coinImage} source={coinInfo.icon} />
            <Text style={styles.description}>{`${localLabelFromBCH(amount, coin)} sent`}</Text>
          </View>
        </View>
        <TouchableWithoutFeedback onPress={this.toTransactionDetails}>
          <View style={styles.buttonWrapper}>
            <Text style={styles.buttonText}>
            {I18n.t('screens.paymentSuccess.transaction_details')} 
            </Text>
          </View>
        </TouchableWithoutFeedback>
      </React.Fragment>
    );
  }

  renderSending = () => (
    <View style={styles.wrapper}>
      <View style={styles.iconWrapper}>
        <Animated.View
          style={[
              styles.loadingWheel,
              {
                transform: [
                  {
                    rotate: this.aniVal.interpolate({
                      inputRange: [0, 1],
                      outputRange: ['0deg', '360deg'],
                    }),
                  },
                ],
              },
            ]}
        />
      </View>
      <Text style={styles.statusTitle}>{I18n.t('screens.paymentSuccess.processing')}</Text>
      <Text style={styles.statusDescription}>{I18n.t('screens.paymentSuccess.hang_tight')}</Text>
    </View>
  )

  renderFailure = () => (
    <View style={styles.wrapper}>
      <View style={{ flex: 1 }} />
      <View style={styles.iconWrapper}>
        <Ionicons name="md-close" color={warningColor} size={110} />
      </View>
      <Text style={styles.statusTitle}>{I18n.t('screens.paymentSuccess.Uh_oh')}</Text>
      <Text style={styles.statusDescription}>
      {I18n.t('screens.paymentSuccess.failed_go_through')}
      </Text>
      <TouchableWithoutFeedback onPress={this.handleRetry}>
        <View style={styles.retryButton} >
          <Text style={styles.retryButtonText}>{I18n.t('screens.paymentSuccess.retry')}</Text>
        </View>
      </TouchableWithoutFeedback>
      <View style={{ flex: 1 }} />
      <Text style={styles.errorDescription}>
        <Text style={styles.errorTitle}>{I18n.t('screens.paymentSuccess.error')}</Text>
        {` ${this.props.reason}`}
      </Text>
    </View>
  )

  renderContent = () => {
    const { status } = this.props;
    switch (status) {
      case 'sending':
        return this.renderSending();
      case 'send_failed':
        return this.renderFailure();
      default:
        return this.renderSuccess();
    }
  }

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          title=""
        />
        {this.renderContent()}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
  status: state.wallet.status,
  reason: state.wallet.reason,
  sentTxid: state.wallet.sentTxid,
});

const mapDispatchToProps = {
  sendFunds,
  resetStatus,
};

export default connect(mapStateToProps, mapDispatchToProps)(PaymentSuccess);
