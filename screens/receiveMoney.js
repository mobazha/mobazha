import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Image, View, Clipboard, Text, TouchableWithoutFeedback, Share, ScrollView } from 'react-native';
import QRCode from 'react-native-qrcode-svg';
import { get, isEmpty } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';

import NavShareButton from '../components/atoms/NavShareButton';
import NavBackButton from '../components/atoms/NavBackButton';
import Header from '../components/molecules/Header';
import CoinTypeSelector from '../components/organism/CoinTypeSelector';
import { primaryTextColor, foregroundColor, borderColor, brandColor } from '../components/commonColors';

import { setReceiveMoneyCoin } from '../reducers/appstate';
import { eventTracker } from '../utils/EventTracker';
import { screenWrapper } from '../utils/styles';
import { COINS } from '../utils/coins';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    alignItems: 'center',
    flex: 1,
  },
  contentWrapper: {
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: foregroundColor,
    borderColor,
    padding: 16,
  },
  coinSelector: {
    marginTop: 8,
    marginHorizontal: 16,
  },
  description: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    marginBottom: 6,
  },
  coinAddress: {
    fontSize: 13,
    paddingTop: 10,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    marginVertical: 4,
  },
  doCopy: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
    marginLeft: 6,
  },
  icon: {
    marginVertical: 10,
    width: 38,
    height: 38,
  },
  button: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 60,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 270,
    top: 200,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

class ReceiveMoney extends PureComponent {
  // eslint-disable-next-line react/sort-comp
  constructor(props) {
    super(props);

    this.state = {
      copied: false,
      coin: props.navigation.getParam('coin') || props.coin,
    };
  }

  getCoinAddress = () => {
    const { coin } = this.state;
    const { coinAddresses } = this.props;

    let coinAddress = get(coinAddresses, `${coin}`, 'Loading...');
    if (isEmpty(coinAddress)) {
      coinAddress = 'Loading ...';
    }
    return coinAddress;
  }

  handleCopyLink = () => {
    const coinAddress = this.getCoinAddress();
    const { coin } = this.state;
    Clipboard.setString(coinAddress);
    eventTracker.trackEvent('Wallet-CopiedReceiveAddress', { coin });
    this.setState({ copied: true });
    setTimeout(() => this.setState({ copied: false }), 2000);
  };

  handleShare = () => {
    const coinAddress = this.getCoinAddress();
    Share.share({
      message: coinAddress,
      title: I18n.t('screens.receiveMoney.share_address'),
    });
  }

  handleGoBack = () => this.props.navigation.goBack()

  handleCoinChanged = (coin) => {
    this.setState({ coin });
    this.props.setReceiveMoneyCoin(coin);
  }

  render() {
    const { copied, coin } = this.state;
    const coinAddress = this.getCoinAddress();
    const { icon } = COINS[coin];

    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.handleGoBack}
          right={<NavShareButton />}
          onRight={this.handleShare}
        />
        <ScrollView style={{ flex: 1 }} contentContainerStyle={styles.wrapper}>
          <Image style={styles.icon} source={icon} resizeMode="cover" />
          <TouchableWithoutFeedback onPress={this.handleCopyLink}>
            <View style={styles.contentWrapper}>
              <QRCode value={coinAddress} size={200} />
              <Text style={styles.coinAddress}>{coinAddress}</Text>
            </View>
          </TouchableWithoutFeedback>
          <TouchableWithoutFeedback onPress={this.handleCopyLink}>
            <View style={styles.button}>
              <Ionicons name="md-copy" color={brandColor} size={24} />
              <Text style={styles.doCopy}>{I18n.t('screens.receiveMoney.copy_address')}</Text>
            </View>
          </TouchableWithoutFeedback>
          <View style={{ flex: 1 }} />
        </ScrollView>
        <CoinTypeSelector coin={coin} onChange={this.handleCoinChanged} />
        {copied && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{I18n.t('screens.receiveMoney.address_copied')} </Text>
          </View>
        )}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  coinAddresses: state.wallet.coinAddress,
  coin: state.appstate.receiveMoneyCoin,
});

const mapDispatchToProps = {
  setReceiveMoneyCoin,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ReceiveMoney);
