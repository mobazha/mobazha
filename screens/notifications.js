import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';
import { NavigationEvents } from 'react-navigation';

import Header from '../components/molecules/Header';
import NotificationTemplate from '../components/templates/Notification';
import { fetchSocialNotifications, markAllAsSeen } from '../reducers/stream';
import { fetchNotifications, markAllAsRead } from '../reducers/notifications';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';
class Notifications extends PureComponent {
  initNotifications = () => {
    this.props.fetchNotifications();
    this.props.fetchSocialNotifications({ appending: false });
  }

  markAllAsRead = () => {
    this.props.markAllAsSeen();
    this.props.markAllAsRead();
  }

  handleNavigationFocus = () => {
    this.initNotifications();
    setTimeout(this.markAllAsRead, 1000);
  }

  handleBack = () => {
    this.props.navigation.goBack();
  }

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.handleBack}
          title={I18n.t('screens.notifications.Notifications')}
        />
        <NavigationEvents onDidFocus={this.handleNavigationFocus} onDidBlur={this.markAllAsRead} />
        <NotificationTemplate />
      </View>
    );
  }
}

const mapDispatchToProps = {
  fetchNotifications,
  fetchSocialNotifications,
  markAllAsSeen,
  markAllAsRead,
};

export default connect(null, mapDispatchToProps)(Notifications);
