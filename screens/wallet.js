import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, ScrollView, RefreshControl } from 'react-native';

import NavBackButton from '../components/atoms/NavBackButton';
import NavOptionButton from '../components/atoms/NavOptionButton';
import BuyWyre from '../components/molecules/BuyWyre';
import TabHeader from '../components/organism/TabHeader';
import OBActionSheet from '../components/organism/ActionSheet';
import WalletTemplate from '../components/templates/Wallet';
import SendReceiveMoney from '../components/templates/SendReceiveMoney';
import { screenWrapper } from '../utils/styles';

import { fetchWalletBalance } from '../reducers/wallet';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

const styles = {
  contentWrapper: {
    alignSelf: 'stretch',
    flex: 1,
  },
};

class Wallet extends PureComponent {
  state = {
    refreshing: false,
  };

  componentDidMount() {
    this.props.fetchWalletBalance();
  }

  onRefresh = () => {
    this.setState({ refreshing: true });
    const { fetchWalletBalance } = this.props;
    fetchWalletBalance();
    this.setState({ refreshing: false });
  };

  setActionSheet = (ref) => { this.actionSheet = ref; }

  handleGoBack = () => this.props.navigation.goBack();

  handleSend = () => {
    eventTracker.trackEvent('WalletTapped-SendMoney');
    this.props.navigation.navigate('SendMoney');
  }

  handleReceive = () => {
    eventTracker.trackEvent('WalletTapped-ReceiveMoney');
    this.props.navigation.navigate('ReceiveMoney');
  }

  handleMoreOption = () => {
    if (this.actionSheet) {
      this.actionSheet.show();
    }
  }

  handleActionSheet = (index) => {
    if (index === 0) {
      eventTracker.trackEvent('WalletTapped-Transactions');
      this.props.navigation.navigate('Transactions');
    }
  }

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <TabHeader
          title={I18n.t('screens.wallet.Wallet')}
          left={<NavBackButton white />}
          onLeft={this.handleGoBack}
          right={<NavOptionButton white />}
          onRight={this.handleMoreOption}
        />
        <ScrollView
          style={styles.contentWrapper}
          contentContainerStyle={styles.contentContainer}
          refreshControl={<RefreshControl
            refreshing={this.state.refreshing}
            onRefresh={this.onRefresh}
          />}
        >
          <WalletTemplate />
          <BuyWyre />
          <View style={{ flex: 1 }} />

        </ScrollView>
        <SendReceiveMoney sendMoney={this.handleSend} receiveMoney={this.handleReceive} />
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleActionSheet}
          options={[I18n.t('screens.wallet.View_history'), I18n.t('screens.wallet.Cancel')]}
          cancelButtonIndex={1}
        />
      </View>
    );
  }
}

const mapDispatchToProps = {
  fetchWalletBalance,
};

export default connect(
  null,
  mapDispatchToProps,
)(Wallet);
