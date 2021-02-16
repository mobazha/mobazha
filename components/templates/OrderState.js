import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, RefreshControl, FlatList } from 'react-native';
import { withNavigation } from 'react-navigation';
import { filter } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import Foundation from 'react-native-vector-icons/Foundation';

import { formLabelColor } from '../commonColors';
import { fetchExchangeRate } from '../../reducers/exchangeRate';
import { fetchWalletBalance } from '../../reducers/wallet';
import { fetchPurchases, fetchSales } from '../../reducers/order';
import { isOrderStatusInCategory } from '../../utils/order';
import { ordersMap } from '../../selectors/order';
import OrderBrief from '../organism/OrderBrief';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
  },
  emptyWrapper: {
    flex: 1,
    backgroundColor: '#ffffff',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 120,
  },
  emptyIcon: {
    marginBottom: 10,
  },
  emptyText: {
    color: '#8a8a8f',
    fontSize: 15,
    width: 294,
    textAlign: 'center',
  },
};

class OrderState extends PureComponent {
  UNSAFE_componentWillMount() {
    this.props.fetchExchangeRate();
    this.props.fetchWalletBalance();
  }

  filterOrders = () => {
    const { orders, category } = this.props;
    return filter(
      orders,
      o => category === 'All' || isOrderStatusInCategory(o.state, category),
    );
  }

  handleGoToOrder = (orderId) => {
    const { navigation, orderType } = this.props;
    eventTracker.trackEvent('Order-ViewOrderDetails');
    navigation.navigate({
      routeName: 'OrderDetails',
      params: {
        orderId,
        orderType,
      },
    });
  }

  renderEmptyContent = () => {
    const { orderType } = this.props;
    const iconName = orderType === 'purchases' ? 'ios-cart' : 'price-tag';
    const Icon = orderType === 'purchases' ? Ionicons : Foundation;
    return (
      <View style={styles.emptyWrapper}>
        <Icon style={styles.emptyIcon} name={iconName} size={50} color={formLabelColor} />
        <Text style={styles.emptyText}>{I18n.t('components.templates.OrderState.no_orders')}</Text>
      </View>
    );
  }

  renderItem = ({ item, index }) => {
    const { orderType } = this.props;

    return (
      <OrderBrief
        orderType={orderType}
        order={item}
        onPress={this.handleGoToOrder}
        highlightUnread
        showOrderId
        topBorder={index === 0}
      />
    );
  }

  render() {
    const { refreshing, onRefresh, header } = this.props;
    const orders = this.filterOrders();
    return (
      <View style={styles.wrapper}>
        <FlatList
          data={orders}
          keyExtractor={(item, index) => `order_${index}`}
          renderItem={this.renderItem}
          ListHeaderComponent={header}
          ListEmptyComponent={this.renderEmptyContent()}
          refreshControl={
            <RefreshControl
              refreshing={refreshing}
              onRefresh={onRefresh}
            />
          }
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  balance: state.wallet.balance,
  ...ordersMap(state),
});

const mapDispatchToProps = {
  fetchExchangeRate,
  fetchWalletBalance,
  fetchSales,
  fetchPurchases,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
)(OrderState));
