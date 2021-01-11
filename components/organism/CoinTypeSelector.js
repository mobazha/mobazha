import React, { PureComponent } from 'react';
import { FlatList, Image, View, Text, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { connect } from 'react-redux';
import { ifIphoneX } from 'react-native-iphone-x-helper';
import * as _ from 'lodash';

import Header from '../molecules/Header';
import { staticLabelColor, brandColor } from '../commonColors';

import { OBLightModal } from '../templates/OBModal';
import { COINS } from '../../utils/coins';
import CoinListItem from '../molecules/CoinListItem';
import NavCloseButton from '../atoms/NavCloseButton';
import { minUnitAmountToBCH } from '../../utils/currency';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    height: ifIphoneX(94, 60),
    paddingBottom: ifIphoneX(34, 0),
    backgroundColor: 'white',
  },
  wrapperBorder: {
    borderTopColor: '#e8e8e8',
    borderTopWidth: 1,
  },
  optionTrigger: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 15,
  },
  leftWrapper: {
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    paddingLeft: 8,
  },
  icon: {
    width: 30,
    height: 30,
  },
  coinName: {
    fontSize: 14,
    fontWeight: 'bold',
    letterSpacing: 0,
    color: '#000000',
  },
  secondary: {
    fontSize: 12,
    letterSpacing: 0,
    marginTop: 2,
    color: staticLabelColor,
  },
  iconUp: {
    paddingRight: 15,
  },
  iconWithBalance: {
    paddingLeft: 13,
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

class CoinTypeSelector extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      coin: props.coin,
      showModal: false,
    };
  }

  keyExtractor = (item, index) => `coin_item_${index}`

  handleShowModal = () => {
    this.setState({ showModal: true });
  };

  handleCoinSelected = (coin) => {
    this.setState(
      { coin, showModal: false },
      () => {
        this.props.onChange(coin);
      },
    );
  }

  renderItem = ({ item }) => {
    const { coin } = this.props;
    const childProps = {
      coin: item,
      onCoinSelected: this.handleCoinSelected,
    };
    return (
      <CoinListItem {...childProps} selected={coin === item} />
    );
  }

  render() {
    const { coin, showModal } = this.state;
    const { localLabelFromLocal, convertBalanceFromBCH, showBalance, noBorder } = this.props;
    const balance = !COINS[coin].disabled && this.props.balance[coin];
    const { label, icon, disabled } = COINS[coin];
    const { cBalance } = convertBalanceFromBCH(balance, coin);

    return (
      <View style={[styles.wrapper, !noBorder && styles.wrapperBorder]}>
        <TouchableWithoutFeedback onPress={this.handleShowModal}>
          <View style={styles.optionTrigger}>
            <Image style={styles.icon} source={icon} resizeMode="cover" />
            <View style={styles.leftWrapper}>
              <Text style={styles.coinName}>{label}</Text>
              <Text style={styles.secondary}>{coin}</Text>
            </View>
            <View style={{ flex: 1 }} />
            {showBalance && (
              <View style={styles.rightWrapper}>
                <Text style={styles.priceByLocalCurrency}>
                  {localLabelFromLocal(cBalance)}
                </Text>
                <Text style={styles.secondary}>
                  {(disabled || !balance) ? I18n.t('components.organism.CoinTypeSelector.coming_soon') : `${minUnitAmountToBCH(balance.confirmed, coin)} ${coin}`}
                </Text>
              </View>
            )}
            <Ionicons
              style={showBalance ? styles.iconWithBalance : styles.iconUp}
              name="ios-arrow-up"
              size={18}
              color={staticLabelColor}
            />
          </View>
        </TouchableWithoutFeedback>
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={() => {}}
        >
          <Header
            modal
            left={<NavCloseButton />}
            onLeft={() => this.setState({ showModal: false })}
          />
          <FlatList
            keyExtractor={this.keyExtractor}
            data={Object.keys(COINS)}
            renderItem={this.renderItem}
          />
        </OBLightModal>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  balance: state.wallet.balance,
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(CoinTypeSelector);
