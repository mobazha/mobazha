import React from 'react';
import { connect } from 'react-redux';
import { FlatList, View, Text, ActivityIndicator } from 'react-native';
import { take, get } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { withNavigation, NavigationEvents } from 'react-navigation';

import Notification from '../organism/Notification';
import Tabs from '../organism/Tabs';

import { fetchSocialNotifications, actions as streamActions } from '../../reducers/stream';
import { fetchNotifications } from '../../reducers/notifications';
import { eventTracker } from '../../utils/EventTracker';
import { filterNotifications } from '../../utils/notification';
import { getNetworkNotifications, getStreamNotifiactions } from '../../selectors/notifications';
import { secondaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  emptyWrapper: {
    marginTop: 110,
    alignItems: 'center',
  },
  emptyText: {
    marginTop: 12,
    width: 311,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
  },
  loading: {
    marginTop: 30,
  },
};

const socialTab = {
  label: I18n.t('components.templates.Notification.Social'),
  value: 'social',
};

const orderTab = {
  label: I18n.t('components.templates.Notification.Orders'),
  value: 'order',
};

const getUnreadOrderCount = notifications => notifications.reduce((prev, current) => {
  if (current.read === false) {
    return prev + 1;
  }
  return prev;
}, 0);

const getUnreadSocialCount = notifications => notifications.reduce((prev, current) => {
  if (current.is_seen === false) {
    return prev + 1;
  }
  return prev;
}, 0);

class NotificationTemplate extends React.PureComponent {
  state = {
    currentTab: 'social',
    endPos: 10,
  };

  onRefresh = () => {
    const { fetchSocialNotifications, fetchNotifications } = this.props;
    fetchSocialNotifications({ appending: true });
    fetchNotifications();
  }

  onChangeTab = (tab) => {
    eventTracker.trackEvent('ViewedNotificationTab', { tab });
    this.setState({ currentTab: tab, endPos: 10 });
  }

  getNotificationCounts = () => {
    const { socialNotifications, networkNotifications } = this.props;
    const socialCount = getUnreadSocialCount(socialNotifications);
    const notifications = filterNotifications(networkNotifications, 'order');
    const orderCount = getUnreadOrderCount(notifications);
    return { socialCount, orderCount };
  }

  getTabs = () => {
    const { socialCount, orderCount } = this.getNotificationCounts();
    return [{ ...socialTab, count: socialCount }, { ...orderTab, count: orderCount }];
  }

  handleNavigationFocus = () => {
    const { navigation } = this.props;
    const { currentTab } = this.state;
    const tab = navigation.getParam('tab');
    this.setState({ currentTab: tab || currentTab });
  }

  filterItems = () => {
    const { socialNotifications, networkNotifications } = this.props;
    const { currentTab } = this.state;
    if (currentTab === 'social') {
      return socialNotifications;
    }
    return filterNotifications(networkNotifications, currentTab);
  };

  loadMore = () => {
    const { currentStatus } = this.props;
    const { endPos } = this.state;
    const notifications = this.filterItems();
    if (notifications.length > endPos && currentStatus !== streamActions.fetchSocialNotifications) {
      this.props.fetchSocialNotifications({ appending: false });
      this.setState({ endPos: endPos + 10 });
    }
  };

  keyExtractor = (item) => {
    const notificationId = get(item, 'notification.notificationId', get(item, 'activities[0].id'));
    return `notification_${notificationId}`;
  }

  socialKeyExtractor = notification => notification.id;

  renderEmptyState = () => {
    const { currentStatus } = this.props;
    const { currentTab } = this.state;
    if (currentTab === 'social' && currentStatus === streamActions.fetchSocialNotifications) {
      return <ActivityIndicator style={styles.loading} size="large" color={secondaryTextColor} />;
    }
    const emptyText = {
      social: I18n.t('components.templates.Notification.social_empty'),
      order: I18n.t('components.templates.Notification.order_empty'),
    };
    return (
      <View style={styles.emptyWrapper}>
        <Ionicons name="ios-notifications" size={50} color="#8a8a8f" />
        <Text style={styles.emptyText}>{emptyText[currentTab]}</Text>
      </View>
    );
  };

  renderItem = ({ item }) => <Notification notification={item} />;

  render() {
    try {
      const { loading } = this.props;
      const { currentTab, endPos } = this.state;
      const notifications = this.filterItems();
      const renderingNotifications = take(notifications, endPos);
      return (
        <View style={{ flex: 1 }}>
          <NavigationEvents onDidFocus={this.handleNavigationFocus} />
          <Tabs
            currentTab={currentTab}
            tabs={this.getTabs()}
            onChange={this.onChangeTab}
          />
          <FlatList
            contentContainerStyle={styles.wrapper}
            data={renderingNotifications}
            renderItem={this.renderItem}
            ListEmptyComponent={this.renderEmptyState()}
            keyExtractor={this.keyExtractor}
            onEndReachedThreshold={0.9}
            onEndReached={this.loadMore}
            extraData={endPos}
            onRefresh={this.onRefresh}
            refreshing={loading}
          />
        </View>
      );
    } catch (error) {
      return false;
    }
  }
}

const mapStateToProps = state => ({
  networkNotifications: getNetworkNotifications(state),
  socialNotifications: getStreamNotifiactions(state),
  loading: state.notifications.loading,
  currentStatus: state.stream.currentStatus,
});

const mapDispatchToProps = { fetchNotifications, fetchSocialNotifications };

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
  null,
  { withRef: true },
)(NotificationTemplate));
