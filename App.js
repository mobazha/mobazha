import React, { PureComponent } from 'react';
import { YellowBox } from 'react-native';
import AsyncStorage from '@react-native-community/async-storage';
import Messaging from '@react-native-firebase/messaging';
import { Provider } from 'react-redux';
import { PersistGate } from 'redux-persist/lib/integration/react';
import { MenuProvider } from 'react-native-popup-menu';
import Countly from 'countly-sdk-react-native-bridge';
import Reactotron from 'reactotron-react-native';

import './ReactotronConfig';
import { serverConfig } from './utils/server';
import store, { persistor } from './store';
import OnboardingWrapper from './OnboardingWrapper';

const ignorableWarnings = [
  'Can only update a mounted or mounting component.',
  'Module RNToastNative requires',
];
YellowBox.ignoreWarnings(ignorableWarnings);

Countly.enableCrashReporting(false);

export default class App extends PureComponent {
  constructor(props) {
    super(props);
    serverConfig.setServerToken(props.server_token);
  }

  async componentDidMount() {
    // this.checkPermission();
  }

  getToken = async () => {
    let fcmToken = await AsyncStorage.getItem('fcmToken');
    if (!fcmToken) {
      fcmToken = await Messaging().getToken();
      if (fcmToken) {
        // user has a device token
        await AsyncStorage.setItem('fcmToken', fcmToken);
      }
    }
    Reactotron.log('------------', fcmToken);
  }

  checkPermission = async () => {
    const enabled = await Messaging().hasPermission();
    if (enabled) {
      this.getToken();
    } else {
      this.requestPermission();
    }
  }

  async requestPermission() {
    try {
      await Messaging().requestPermission();
      this.getToken();
    } catch (error) {
      console.log('permission rejected');
    }
  }

  render() {
    return (
      <Provider store={store}>
        <PersistGate persistor={persistor}>
          <MenuProvider>
            <OnboardingWrapper />
          </MenuProvider>
        </PersistGate>
      </Provider>
    );
  }
}
