import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, ScrollView, RefreshControl } from 'react-native';

import Header from '../components/molecules/Header';
import CoinBalance from '../components/organism/CoinBalance';
import TransactionHistory from '../components/templates/TransactionHistory';
import SendReceiveMoney from '../components/templates/SendReceiveMoney';
import { screenWrapper } from '../utils/styles';

import { fetchTransactions, fetchWalletBalance } from '../reducers/wallet';

import NavBackButton from '../components/atoms/NavBackButton';
import { COINS } from '../utils/coins';
import QRScanner from '../components/organism/QRScanner';

import {I18n} from '../langs/I18n';

const styles = {
  rightIcon: {
    padding: 10,
  },
};

class CryptoBalance extends PureComponent {
  state = { refreshing: false };

  UNSAFE_componentWillMount() {
    const coin = this.props.navigation.getParam('coin');
    this.props.fetchTransactions(coin);
  }

  componentDidUpdate(prevProps) {
    const { fetchingBalance, fetchingTransactions } = this.props;
    const { refreshing } = this.state;
    if (prevProps.fetchingBalance !== fetchingBalance ||
      prevProps.fetchingTransactions !== fetchingTransactions) {
      if (refreshing) {
        if (!fetchingBalance && !fetchingTransactions) {
          this.setState({ refreshing: false });
        }
      }
    }
  }

  onRefresh = () => {
    const { navigation, fetchWalletBalance, fetchTransactions } = this.props;
    this.setState({ refreshing: true });
    fetchWalletBalance();
    const coin = navigation.getParam('coin');
    fetchTransactions(coin);
  };

  onLeft = () => {
    this.props.navigation.goBack();
  }

  onSendMoney = () => {
    const { navigation } = this.props;
    const coin = navigation.getParam('coin');
    navigation.navigate({
      routeName: 'SendMoney',
      params: { coin },
    });
  }

  onReceiveMoney = () => {
    const { navigation } = this.props;
    const coin = navigation.getParam('coin');
    navigation.navigate({
      routeName: 'ReceiveMoney',
      params: { coin },
    });
  }

  render() {
    const { balance, navigation } = this.props;
    const coin = navigation.getParam('coin');
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          navigation={navigation}
          onLeft={this.onLeft}
          title={COINS[coin].label}
          right={<QRScanner style={styles.rightIcon} black fromWallet />}
        />
        <ScrollView
          refreshControl={
            <RefreshControl
              refreshing={this.state.refreshing}
              onRefresh={this.onRefresh}
            />
          }
        >
          <CoinBalance balance={balance} coin={coin} />
          <View style={{ flex: 1 }}>
            <TransactionHistory coin={coin} />
          </View>
        </ScrollView>
        <SendReceiveMoney
          sendMoney={this.onSendMoney}
          receiveMoney={this.onReceiveMoney}
        />
      </View>
    );
  }
}

const mapStateToProps = (state, props) => {
  const coin = props.navigation.getParam('coin');
  return {
    balance: state.wallet.balance[coin],
    fetchingTransactions: state.wallet.fetchingTransactions,
    fetchingBalance: state.wallet.fetchingBalance,
  };
};

const mapDispatchToProps = { fetchTransactions, fetchWalletBalance };

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(CryptoBalance);
