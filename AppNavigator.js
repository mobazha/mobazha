import React from 'react';
import { connect } from 'react-redux';
import { BackHandler, View, Platform } from 'react-native';
import Branch from 'react-native-branch';
import { NavigationActions, StackActions } from 'react-navigation';
import Spinner from 'react-native-loading-spinner-overlay';
import Messaging from '@react-native-firebase/messaging';
import Reactotron from 'reactotron-react-native';

import AppContainer from './routes';
import { getNavRouteAndParamsFromURL, getRouteNameFromState, getCurrentRouteParamFromState } from './utils/navigation';
import { fetchChats } from './reducers/chat';
import { clearSettingsError } from './reducers/settings';
import { clearProfileError } from './reducers/profile';
import ErrorModal from './components/organism/ErrorModal';
import PlusPanelView from './components/organism/PanelView/PlusPanelView';
import SharePanelView from './components/organism/PanelView/SharePanelView';
import ToastTemplate from './components/templates/Toast';

import {I18n} from './langs/I18n';

const styles = {
  spinnerText: {
    color: 'white',
  },
};

class AppNavigatorRedux extends React.PureComponent {
  state = {
  }

  constructor(props) {
    super(props);
    this.currentScreen = this.props.nav.routes[this.props.nav.index].routeName;
  }

  componentDidMount() {
    BackHandler.addEventListener('hardwareBackPress', () => {
      const { nav } = this.props;
      if (
        nav.routes.length === 1 &&
        (nav.routes[0].routeName === 'Login' || nav.routes[0].routeName === 'Start')
      ) {
        return false;
      }
      // if (shouldCloseApp(nav)) return false
      let backAction = NavigationActions.back();
      if (this.navigator) {
        const { screenKey } = getCurrentRouteParamFromState(this.navigator.state.nav);
        if (screenKey) {
          backAction = NavigationActions.back({ key: screenKey });
        }
        this.navigator.dispatch(backAction);
      }
      return true;
    });

    this.subscribeBranch();

    // this.createNotificationListeners();
  }

  componentWillUnmount() {
    BackHandler.removeEventListener('hardwareBackPress');

    if (this.unsubscribeFromBranch) {
      this.unsubscribeFromBranch();
      this.unsubscribeFromBranch = null;
    }

    if (this.removeNotificationListener) {
      this.removeNotificationListener();
    }

    if (this.removeNotificationOpenedListener) {
      this.removeNotificationOpenedListener();
    }
  }

  onNavigationStateChange = (prevState, newState, action) => {
    const routeName = getRouteNameFromState(newState);
    this.currentScreen = routeName;
    const { fetchChats } = this.props;
    if (
      (action.type === 'Navigation/NAVIGATE' || action.type === 'Navigation/COMPLETE_TRANSITION') &&
      routeName === 'ChatsScreen'
    ) {
      fetchChats();
    }
  };

  subscribeBranch = () => {
    this.unsubscribeFromBranch = Branch.subscribe(({ error, params }) => {
      if (error) {
        console.error('Branch Error:', error);
        return;
      }

      const { setInviter } = this.props;
      let url;

      if (params.inviter) {
        setInviter(params.inviter);
        return;
      } else if (params['+non_branch_link']) {
        url = params['+non_branch_link'];
      } else if (params['+clicked_branch_link'] && params.deeplink) {
        url = params.deeplink;
      } else {
        // Initialization success but neither link was opened nor inviter param was passed
        return;
      }
      if (url) {
        const { route, params } = getNavRouteAndParamsFromURL(url) || {};
        if (route) {
          switch (route) {
            case 'Feed':
              this.navigator.dispatch(NavigationActions.navigate({ routeName: 'Feed' }));
              break;
            default:
              this.navigator.dispatch(StackActions.push({
                routeName: route, params: { ...params },
              }));
              break;
          }
        }
      }
    });
  };

  handlePushNavigation = (type, activityId, orderId, orderType) => {
    switch (type) {
      case 'chat': {
        if (orderId) {
          this.navigator.dispatch(NavigationActions.navigate({
            routeName: 'OrderDetails',
            params: {
              orderId,
              orderType,
              tab: 'discussion',
            },
          }));
        } else {
          this.navigator.dispatch(NavigationActions.navigate({ routeName: 'Chat' }));
        }
        break;
      }
      case 'order': {
        this.navigator.dispatch(StackActions.push({
          routeName: 'Notifications',
          params: {
            tab: 'order',
          },
        }));
        break;
      }
      case 'repost':
      case 'comment': {
        this.navigator.dispatch(StackActions.push({
          routeName: 'FeedDetail',
          params: {
            activityId, tab: 'comment', showKeyboard: false,
          },
        }));
        break;
      }
      case 'like': {
        this.navigator.dispatch(StackActions.push({
          routeName: 'FeedDetail',
          params: {
            activityId, tab: 'like', showKeyboard: false,
          },
        }));
        break;
      }
      case 'follow': {
        this.navigator.dispatch(StackActions.push({
          routeName: 'Followers',
        }));
        break;
      }
      default:
        break;
    }
  }

  async createNotificationListeners() {
    // this.removeNotificationListener = Messaging().onNotification((notification) => {
    //   Reactotron.log('-------------onNotification-------------', notification);
    //   if (notification.data && notification.data.type === 'chat' &&
    //     this.currentScreen.toLowerCase().includes('chat')) {
    //     // if this is a chat notification and the viewer is on a chat screen, do not display it.
    //     return;
    //   }
    //   notification
    //     .android.setChannelId('fcm_fallback_notification_channel')
    //     .android.setSmallIcon('ic_launcher')
    //     .android.setPriority(Firebase.notifications.Android.Priority.Max)
    //     .android.setAutoCancel(true)
    //     .setSound('default');
    //   Messaging().displayNotification(notification);
    // });

    this.removeNotificationOpenedListener = Messaging().onNotificationOpenedApp((notificationOpen) => {
      Reactotron.log('-------------onNotificationOpened-------------', notificationOpen);
      const { data, notificationId } = notificationOpen.notification;
      const { type, activityId, orderId, orderType } = data || {};

      Reactotron.log('--------notificationId, type, activityId--------', notificationId, type, activityId);

      this.handlePushNavigation(type, activityId, orderId, orderType);
    });

    this.messageListener = Messaging().onMessage((message) => {
      // process data message
      Reactotron.log('-------------onMessage-------------', message);
    });

    if (Platform.OS === 'android') {
      const notificationOpen = await Messaging().getInitialNotification();
      if (notificationOpen) {
        Reactotron.log('-------------getInitialNotification-------------', notificationOpen);
        const { data, notificationId } = notificationOpen.notification;
        const { type, activityId, orderId, orderType  } = data;
        Reactotron.log('--------notificationId, type, activityId--------', notificationId, type, activityId);

        this.handlePushNavigation(type, activityId, orderId, orderType);
      }
    }
  }

  render() {
    const { spinner, settingsError, profileError } = this.props;
    let errorSource;
    let onPress;
    if (settingsError) {
      errorSource = 'settings';
      onPress = this.props.clearSettingsError;
    }
    if (profileError) {
      errorSource = 'profile';
      onPress = this.props.clearProfileError;
    }
    const errorProps = {
      title: `There was an issue while updating your ${errorSource}.`,
      error: this.props[`${errorSource}Error`],
      buttonText: 'OK',
      onPress,
    };

    return (
      <View style={{ flex: 1 }}>
        <Spinner
          visible={spinner}
          textContent={I18n.t('AppNavigator.Please_wait')}
          textStyle={styles.spinnerText}
        />
        <AppContainer
          ref={(navigatorRef) => {
            this.navigator = navigatorRef;
            this.setState({ navigator: navigatorRef });
          }}
          onNavigationStateChange={this.onNavigationStateChange}
        />
        <SharePanelView />
        <PlusPanelView navigator={this.state.navigator} />
        <ToastTemplate navigator={this.state.navigator} />
        {errorSource && <ErrorModal {...errorProps} />}
      </View>
    );
  }
}

AppNavigatorRedux.propTypes = {};

const mapStateToProps = state => ({
  nav: state.nav,
  spinner: state.orders.processing,
  settingsError: state.settings.settingsError,
  profileError: state.profile.profileError,
});

const mapDispatchToProps = {
  fetchChats,
  clearSettingsError,
  clearProfileError,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(AppNavigatorRedux);
