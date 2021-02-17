import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { FlatList, ScrollView, Text, View, Image } from 'react-native';
import { isEmpty, upperCase, reduce, hasIn } from 'lodash';
import moment from 'moment';

import TransactionItem from '../atoms/TransactionItem';
import TransactionIcon from '../../assets/icons/transaction_icon.png';
import { convertorsMap } from '../../selectors/currency';
import { COINS } from '../../utils/coins';
import { renderTextHeader } from '../molecules/TextHeader';

import {I18n} from '../../langs/I18n';

const styles = {
  emptyText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#8a8a8f',
    marginVertical: 30,
    marginHorizontal: 15,
    textAlign: 'center',
  },
  emptyWrapper: {
    marginTop: 200,
    alignItems: 'center',
    marginHorizontal: 15,
    borderBottomWidth: 1,
    borderBottomColor: '#e8e8e8',
  },
  transactionIcon: {
    width: 50,
    height: 36,
  },
  centered: {
    textAlign: 'center',
  },
  txnNotice: {
    fontSize: 11,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#8a8a8f',
    marginVertical: 20,
    marginHorizontal: 10,
    textAlign: 'center',
  },
};

class TransactionHistory extends PureComponent {
  keyExtractor = (item, index) => `transaction_item_${index}`

  renderItem = ({ item }) => {
    const { localSymbol, convertCurrencyFromBCH } = this.props;
    const { coin, transaction } = item;
    const {
      value, txid, timestamp, status,
    } = transaction;
    const coinName = upperCase(coin);
    const amount = convertCurrencyFromBCH(value, coinName);
    const timeDiff = moment.duration(moment().diff(moment(timestamp)));
    let timeStr = '';
    if (timeDiff.asYears() > 1) {
      timeStr = `${Math.floor(timeDiff.asYears())}y`;
    } else if (timeDiff.asMonths() > 1) {
      timeStr = `${Math.floor(timeDiff.asMonths())}mo`;
    } else if (timeDiff.asWeeks() > 1) {
      timeStr = `${Math.floor(timeDiff.asWeeks())}w`;
    } else if (timeDiff.asDays() > 1) {
      timeStr = `${Math.floor(timeDiff.asDays())}d`;
    } else if (timeDiff.asHours() > 1) {
      timeStr = `${Math.floor(timeDiff.asHours())}h`;
    } else if (timeDiff.asMinutes() > 1) {
      timeStr = `${Math.floor(timeDiff.asMinutes())}m`;
    } else if (timeDiff.asSeconds() > 1) {
      timeStr = `${Math.floor(timeDiff.asSeconds())}s`;
    }
    return (
      <TransactionItem
        id={txid}
        valueInCrypto={value}
        amount={amount}
        status={status}
        currencySymbol={localSymbol}
        coin={coinName}
        time={timeStr}
        icon={COINS[coinName].icon}
        key={`transaction_${txid}`}
      />
    );
  }

  renderEmptyState = () => {
    const { solo } = this.props;
    if (solo) {
      return (
        <View style={styles.emptyWrapper}>
          <Image style={styles.transactionIcon} source={TransactionIcon} />
          <Text style={[styles.emptyText, styles.centered]}>
            {I18n.t('components.templates.TransactionHistory.no_transaction_recorded')}
          </Text>
        </View>
      );
    }
    return <Text style={styles.emptyText}>{I18n.t('components.templates.TransactionHistory.no_transactions')}</Text>;
  };

  renderTransactions = () => {
    const { transactions, coin } = this.props;
    const coinsToShow = coin ? [coin] : Object.keys(COINS).filter(key => !COINS[key].disabled);
    const items = reduce(
      coinsToShow,
      (result, coin) => {
        if (hasIn(transactions, `[${coin}].transactions`)) {
          return [
            ...result,
            ...transactions[coin].transactions.map(val => ({
              coin,
              transaction: val,
            })),
          ];
        }
        return result;
      },
      [],
    );
    return isEmpty(items) ? (
      this.renderEmptyState()
    ) : (
      <FlatList
        ListHeaderComponent={renderTextHeader(I18n.t('components.templates.TransactionHistory.Transactions'))}
        keyExtractor={this.keyExtractor}
        data={items}
        renderItem={this.renderItem}
      />
    );
  };
  render() {
    const { coin } = this.props;
    return (
      <ScrollView>{this.renderTransactions()}
        {coin === 'ETH' && (
          <Text style={[styles.txnNotice, styles.centered]}>
            {I18n.t('components.templates.TransactionHistory.notes')}
          </Text>
        )}
      </ScrollView>);
  }
}

const mapStateToProps = state => ({
  transactions: state.wallet.transactions,
  ...convertorsMap(state),
});

const mapDispatchToProps = {};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(TransactionHistory);
