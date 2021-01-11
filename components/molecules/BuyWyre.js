import React from 'react';
import { View, Text, Image, TouchableWithoutFeedback, Platform, Linking } from 'react-native';
import Entypo from 'react-native-vector-icons/Entypo';
import { withNavigation } from 'react-navigation';
import queryString from 'query-string';
import InAppBrowser from 'react-native-inappbrowser-reborn';
import { connect } from 'react-redux';

import WyreLogo from '../../assets/images/wyre_logo_2.png';
import { primaryTextColor, formLabelColor, borderColor, brandColor } from '../commonColors';

import CoinSelectorModal from '../organism/CoinSelectorModal';

import {I18n} from '../../langs/I18n';

const IN_APP_BROWSER_OPTIONS = {
  // iOS Properties
  dismissButtonStyle: 'cancel',
  preferredBarTintColor: brandColor,
  preferredControlTintColor: 'white',
  readerMode: false,
  animated: true,
  modalPresentationStyle: 'fullScreen',
  modalEnabled: true,
  enableBarCollapsing: false,

  // Android Properties
  showTitle: true,
  toolbarColor: brandColor,
  secondaryToolbarColor: 'black',
  enableUrlBarHiding: true,
  enableDefaultShare: true,
  forceCloseOnRedirection: false,
  // Specify full animation resource identifier(package:anim/name)
  // or only resource name(in case of animation bundled with app).
  animations: {
    startEnter: 'slide_in_right',
    startExit: 'slide_out_left',
    endEnter: 'slide_in_left',
    endExit: 'slide_out_right',
  },
};

const styles = {
  container: {
    padding: 16,
    flexDirection: 'column',
  },
  checkoutContainer: {
    paddingBottom: 16,
    flexDirection: 'column',
  },
  wrapper: {
    paddingLeft: 16,
    paddingRight: 8,
    paddingVeritical: 16,
    height: 68,
    flexDirection: 'row',
    alignItems: 'center',
    borderWidth: 1,
    borderColor,
    borderRadius: 4,
  },
  logo: {
    marginRight: 8,
    height: 36,
    width: 36,
  },
  wyreWrapper: {
    height: 36,
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    paddingVertical: 3,
  },
  wyreTitle: {
    flex: 1,
    fontSize: 14,
    fontWeight: 'bold',
    letterSpacing: 0,
    lineHeight: 15,
    color: primaryTextColor,
  },
  wyreText: {
    flex: 1,
    fontSize: 12,
    color: formLabelColor,
    letterSpacing: 0,
    lineHeight: 15,
  },
  icon: {
    paddingRight: 0,
    fontWeight: 'bold',
  },
};

class BuyWyre extends React.PureComponent {
  getPaymentUrl = (coin) => {
    const { coinAddress } = this.props;
    const paymentMethod = Platform.OS === 'ios' ? 'apple-pay' : 'debit-card';
    const walletAddress = coinAddress[coin];
    const qs = queryString.stringify({
      paymentMethod,
      dest: coin === 'BTC' ? `bitcoin:${walletAddress}` : `ethereum:${walletAddress}`,
      destCurrency: coin,
      accountId: 'AC_DMMBYMLG8A2',
    });
    return `https://pay.sendwyre.com?${qs}`;
  }

  setCoinSelectorRef = (ref) => { this.coinSelector = ref; }

  handleBuyWyre = () => {
    this.coinSelector.showModal();
  }

  handleCoinSelect = (coin = 'BTC') => {
    setTimeout(async () => {
      try {
        const url = this.getPaymentUrl(coin);
        if (await InAppBrowser.isAvailable()) {
          await InAppBrowser.open(url, IN_APP_BROWSER_OPTIONS);
        } else {
          Linking.openURL(url);
        }
      } catch (error) {
        console.error(error);
      }
    }, 500);
  }

  render() {
    const { isCheckout } = this.props;
    return (
      <View style={isCheckout ? styles.checkoutContainer : styles.container}>
        <TouchableWithoutFeedback onPress={this.handleBuyWyre}>
          <View style={styles.wrapper}>
            <Image source={WyreLogo} style={styles.logo} />
            <View style={styles.wyreWrapper}>
              <Text style={styles.wyreTitle}>{I18n.t('components.molecules.BuyWyre.ask_crypto')}</Text>
              <Text style={styles.wyreText}>{I18n.t('components.molecules.BuyWyre.top_up')}</Text>
            </View>
            <View style={{ flex: 1 }} />
            <Entypo name="chevron-small-right" size={28} color={primaryTextColor} style={styles.icon} />
          </View>
        </TouchableWithoutFeedback>
        <CoinSelectorModal ref={this.setCoinSelectorRef} coin="BTC" onChange={this.handleCoinSelect} />
      </View>
    );
  }
}

const mapStateToProps = ({ wallet: { coinAddress } }) => ({ coinAddress });

export default withNavigation(connect(mapStateToProps)(BuyWyre));
