/* eslint-disable react/sort-comp */
/* eslint-disable comma-dangle */
/* eslint-disable prefer-spread */

import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';
import * as _ from 'lodash';

import orderStatusEn from '../config/orderStatus.json';
import orderStatusZh from '../config/zh/orderStatus.json';

import Header from '../components/molecules/Header';
import { fetchPurchases, fetchSales, scanOfflineMessages } from '../reducers/order';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';
import OrderCategorySelector from '../components/templates/OrderCategorySelector';
import OrderState from '../components/templates/OrderState';
import Tabs from '../components/organism/Tabs';
import { eventTracker } from '../utils/EventTracker';
import { timeSinceInSeconds } from '../utils/time';
import { resyncBlockchain } from '../api/wallet';

import {I18n} from '../langs/I18n';

let orderStatus = orderStatusEn
if (I18n.locale.startsWith('zh')) {
  orderStatus = orderStatusZh
}

const AWAITING_PAYMENT_EXPIRE_IN_HOURS = 24;

const orderTabs = [
  {
    value: 'purchases',
    label: I18n.t('screens.order.purchases'),
  },
  {
    value: 'sales',
    label: I18n.t('screens.order.sales'),
  },
];

class OrderManagment extends PureComponent {
  constructor(props) {
    super(props);
    const orderType = this.props.navigation.getParam('orderType');
    this.state = { orderType, category: 'All', refreshingPurchases: false, refreshingSales: false };

    this.fetchData();
  }

  fetchData = async () => {
    const { fetchSales, fetchPurchases, scanOfflineMessages } = this.props;

    await resyncBlockchain();
    fetchSales({ shouldFetchProfiles: true });
    fetchPurchases({ shouldFetchProfiles: true });
    scanOfflineMessages();
  }

  onChangeCategory = (category) => {
    eventTracker.trackEvent('Order-ChangeFilter', { filter: category });
    this.setState({ category });
  }

  getFilteredOrders = () => {
    const { orderType } = this.state;
    let orders = [];
    switch (orderType) {
      case 'sales':
        orders = [...this.props.sales];
        break;
      case 'purchases':
        orders = [...this.props.purchases];
        break;
      default:
        break;
    }

    orders = _.sortBy(orders, 'read');

    const allowedStates = [].concat.apply(
      [],
      Object.values(orderStatus)
        .filter(category => category.orderType.includes(orderType))
        .map(category => category.items),
    ).map(item => item.value);

    return orders
      .filter(order => allowedStates.includes(order.state))
      .filter(
        order => order.state !== (timeSinceInSeconds(new Date(order.timestamp)) < AWAITING_PAYMENT_EXPIRE_IN_HOURS * 3600)
      );
  }

  updateOrderTypeFilter = (orderType) => {
    this.setState({ orderType, category: 'All' });
  };

  handleRefreshPurchases = () => {
    this.setState({ refreshingPurchases: true });
    this.props.fetchPurchases({
      shouldFetchProfiles: true,
      onComplete: () => {
        this.setState({ refreshingPurchases: false });
      },
    });
  }

  handleRefreshSales = () => {
    this.setState({ refreshingSales: true });
    this.props.fetchSales({
      shouldFetchProfiles: true,
      onComplete: () => {
        this.setState({ refreshingSales: false });
      },
    });
  }

  render() {
    const {
      orderType, category, refreshingPurchases, refreshingSales,
    } = this.state;
    const orders = this.getFilteredOrders();
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          title={I18n.t('screens.order.orders')}
        />
        <Tabs
          currentTab={orderType}
          tabs={orderTabs}
          onChange={this.updateOrderTypeFilter}
          withBorder
        />
        <OrderState
          header={
            <OrderCategorySelector
              type={orderType}
              orders={orders}
              category={category}
              onChange={this.onChangeCategory}
            />
          }
          orders={orders}
          orderType={orderType}
          category={category}
          refreshing={orderType === 'purchases' ? refreshingPurchases : refreshingSales}
          onRefresh={orderType === 'purchases' ? this.handleRefreshPurchases : this.handleRefreshSales}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  sales: state.orders.sales,
  purchases: state.orders.purchases,
});

const mapDispatchToProps = {
  fetchPurchases,
  fetchSales,
  scanOfflineMessages,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(OrderManagment);
