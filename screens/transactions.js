import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';

import Header from '../components/molecules/Header';
import TransactionHistory from '../components/templates/TransactionHistory';
import { fetchTransactions } from '../reducers/wallet';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';
class Transactions extends PureComponent {
  UNSAFE_componentWillMount() {
    this.props.fetchTransactions();
  }
  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          title="Transactions"
        />
        <TransactionHistory solo />
      </View>
    );
  }
}

const mapStateToProps = state => ({});

const mapDispatchToProps = {
  fetchTransactions,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Transactions);
