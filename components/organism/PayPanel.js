import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { TouchableOpacity, Image, View, Text, Platform } from 'react-native';
import EvilIcons from 'react-native-vector-icons/EvilIcons';
import BigNumber from 'bignumber.js';

import { brandColor, formLabelColor, borderColor, warningColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const logoImg = require('../../assets/images/logo/brand-logo.png');

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
    alignItems: 'center',
  },
  question: {
    marginVertical: 20,
    color: formLabelColor,
    fontSize: 16,
  },
  content: {
    flexDirection: 'row',
    justifyContent: 'center',
  },
  item: {
    flex: 1,
  },
  itemContent: {
    alignItems: 'center',
    paddingVertical: 15,
  },
  leftBorder: {
    borderLeftWidth: 1,
    borderColor,
  },
  imageContainer: {
    width: 56,
    height: 56,
    backgroundColor: '#e8f7e7',
    borderRadius: 28,
    overflow: 'hidden',
    justifyContent: 'center',
    alignItems: 'center',
    ...Platform.select({
      ios: { paddingTop: 5 },
      android: { paddingTop: 0 },
    }),
  },
  logo: {
    width: 44,
    height: 22,
  },
  externalImageGap: {
    // paddingTop: 5,
  },
  title: {
    marginTop: 12,
    fontSize: 14,
    color: '#404040',
  },
  insufficient: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: 'center',
    alignItems: 'center',
  },
  insufficientText: {
    color: warningColor,
    fontSize: 12,
  },
};

class PayPanel extends PureComponent {
  render() {
    const {
      balance, amount, paymentMethod, onInternal, onExternal,
    } = this.props;
    const { confirmed, unconfirmed } = balance[paymentMethod];
    const insufficient = !balance[paymentMethod] || (BigNumber(amount).gt(BigNumber(confirmed).plus(BigNumber(unconfirmed))));
    return (
      <View style={styles.wrapper}>
        <Text style={styles.question}>{I18n.t('components.organism.PayPanel.ask_pay')}</Text>
        <View style={styles.content}>
          <TouchableOpacity style={[styles.item, styles.itemContent]} onPressIn={onExternal} disabled={paymentMethod === 'ETH'}>
            <View style={styles.imageContainer}>
              <EvilIcons
                name="external-link"
                size={40}
                color={brandColor}
              />
            </View>
            <Text style={styles.title}>{I18n.t('components.organism.PayPanel.external_wallet')}</Text>
            {paymentMethod === 'ETH' && (
              <View style={styles.insufficient}>
                <Text style={styles.insufficientText}>{I18n.t('components.organism.PayPanel.not_available_eth')}</Text>
              </View>
            )}
          </TouchableOpacity>
          <TouchableOpacity style={[styles.item, styles.leftBorder]} onPressIn={onInternal} disabled={insufficient}>
            <View style={[styles.itemContent, insufficient ? { opacity: 0.5 } : null]}>
              <View style={styles.imageContainer}>
                <Image style={styles.logo} source={logoImg} resizeMode="stretch" />
              </View>
              <Text style={styles.title}>{I18n.t('components.organism.PayPanel.haven_wallet')}</Text>
            </View>
            {insufficient && (
              <View style={styles.insufficient}>
                <Text style={styles.insufficientText}>{I18n.t('components.organism.PayPanel.not_enough_funds')}</Text>
              </View>
            )}
          </TouchableOpacity>
        </View>
      </View>
    );
  }
}


const mapStateToProps = state => ({
  balance: state.wallet.balance,
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(PayPanel);
