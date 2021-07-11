import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, Image, Alert, TouchableWithoutFeedback } from 'react-native';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import CheckGroup from '../components/atoms/CheckGroup';
import NavBackButton from '../components/atoms/NavBackButton';
import { primaryTextColor, brandColor, borderColor } from '../components/commonColors';

import { updateAcceptedCoins } from '../reducers/profile';

import { getRenderingCoins, COINS } from '../utils/coins';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flexGrow: 1,
    flexDirection: 'row',
    alignItems: 'center',
    height: 60,
  },
  header: {
    fontSize: 15,
    fontWeight: 'bold',
    color: 'black',
    marginLeft: 16,
    marginTop: 24,
    marginBottom: 16,
  },
  coinImage: {
    width: 22,
    height: 22,
    marginRight: 6,
  },
  label: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  selectedLabel: {
    color: brandColor,
    fontWeight: 'bold',
  },
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    borderTopWidth: 1,
    borderColor,
    height: 60,
    paddingHorizontal: 20,
    marginBottom: ifIphoneX(34, 0),
  },
  selectedCount: {
    color: 'black',
    fontSize: 15,
  },
  clearAll: {
    fontSize: 15,
    fontWeight: 'bold',
    color: brandColor,
  },
};

class AcceptedCoins extends PureComponent {
  constructor(props) {
    super(props);
    const { currencies } = props;

    const stateDict = {};
    currencies.forEach((currency) => {
      stateDict[currency] = true;
    });

    this.state = stateDict;
  }

  getSelectedCurrencies = () => Object.keys(COINS).filter(coin => this.state[coin]);

  handleChange = (item, selected) => {
    this.setState({ [item.value]: selected });
  };

  handleGoBack = () => this.props.navigation.goBack();

  handleClear = () => {
    const stateDict = {};
    Object.keys(COINS).forEach((currency) => {
      stateDict[currency] = false;
    });

    this.setState(stateDict);
  };

  handleSave = () => {
    Alert.alert('Update Listings?', 'All your listings will be updated. Are you sure?', [
      { text: I18n.t('screens.acceptedCoins.cancel') },
      {
        text: I18n.t('screens.acceptedCoins.OK'),
        onPress: () => {
          this.props.updateAcceptedCoins({
            coins: this.getSelectedCurrencies(),
            onSuccess: this.handleGoBack,
          });
        },
      },
    ]);
  };

  comparer = (a, b) => a.value === b;

  renderCoin = (coin) => {
    const isSelected = this.state[coin.value];
    return (
      <View style={styles.wrapper}>
        <Image source={coin.icon} style={styles.coinImage} />
        <Text style={[styles.label, isSelected && styles.selectedLabel]}>
          {coin.label} ({coin.value})
        </Text>
      </View>
    );
  };

  render() {
    const selectedCurrencies = this.getSelectedCurrencies();
    const renderingCoins = getRenderingCoins(selectedCurrencies);
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.handleGoBack}
          title= {I18n.t('screens.acceptedCoins.coins_accepted')}
          right={<LinkText text={I18n.t('screens.acceptedCoins.save')} />}
          onRight={this.handleSave}
        />
        <Text style={styles.header}> {I18n.t('screens.acceptedCoins.coins_accepted')} </Text>
        <CheckGroup
          options={renderingCoins}
          selected={selectedCurrencies}
          onChange={this.handleChange}
          renderItem={this.renderCoin}
          compare={this.comparer}
        />
        <View style={styles.footer}>
          <Text style={styles.selectedCount}>{selectedCurrencies.length > 0 ? `${selectedCurrencies.length} ${I18n.t('screens.acceptedCoins.selected')}` : ''}</Text>
          <TouchableWithoutFeedback onPress={this.handleClear}>
            <View>
              <Text style={styles.clearAll}> {I18n.t('screens.acceptedCoins.clear_all')}</Text>
            </View>
          </TouchableWithoutFeedback>
        </View>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  currencies: state.profile.data.currencies,
});

const mapDispatchToProps = {
  updateAcceptedCoins,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(AcceptedCoins);
