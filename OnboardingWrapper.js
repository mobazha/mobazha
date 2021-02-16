import React, { PureComponent } from 'react';
import { Text, DeviceEventEmitter, NativeEventEmitter, NativeModules, Platform, Alert } from 'react-native';
import SplashScreen from 'react-native-splash-screen';
import { connect } from 'react-redux';
import Countly from 'countly-sdk-react-native-bridge';

import { getConfig, createProfile, createSettings } from './api/onboarding';
import { getProfile } from './api/profile';
import { getSettings } from './api/settings';

import { setOnboardingStatus, initializeLogin, triggerReviewPrompt, appInstalled, setGuest } from './reducers/appstate';
import { setProfile, updateAcceptedCoins } from './reducers/profile';
import { setSettings } from './reducers/settings';

import AppNavigator from './AppNavigator';
import { OnboardingContainer } from './routes';
import { changeTrackingStatus } from './utils/EventTracker';
import { COUNTLY_ROOT_URL, COUNTLY_APP_KEY } from './CountlyConfig';
import { COINS } from './utils/coins';

import {I18n} from './langs/I18n';

const { ModuleWithEmitter } = NativeModules;
const iOSeventEmitter = new NativeEventEmitter(ModuleWithEmitter);

class OnboardingWrapper extends PureComponent {
  constructor(props) {
    super(props);

    Countly.init(COUNTLY_ROOT_URL, COUNTLY_APP_KEY).then(() => {
      changeTrackingStatus(props.isTrackingEvent);
    }).catch((err) => {
      // console.error('Countly failed to begin!', err);
    });
  }

  async UNSAFE_componentWillMount() {
    const { setOnboardingStatus } = this.props;
    try {
      await getConfig();
      this.handleServerStarted();
    } catch (err) {
      if (Platform.OS === 'ios') {
        iOSeventEmitter.addListener('onServerStarted', this.handleServerStarted);
      } else {
        DeviceEventEmitter.addListener('onServerStarted', this.handleServerStarted);
      }

      setOnboardingStatus('offline');
    }
  }

  componentDidMount() {
    const { triggerReviewPrompt, isAppInstalled } = this.props;
    if (isAppInstalled) {
      triggerReviewPrompt();
    }
  }

  handleServerStarted = async () => {
    if (Platform.OS === 'ios') {
      iOSeventEmitter.removeListener('onServerStarted', this.handleServerStarted);
    } else {
      DeviceEventEmitter.removeListener('onServerStarted', this.handleServerStarted);
    }

    const {
      guest,
      setSettings,
      setProfile,
      setOnboardingStatus,
      initializeLogin,
      isAppInstalled,
      setGuest,
      updateAcceptedCoins,
      currencies,
    } = this.props;

    try {
      const profile = await getProfile();
      const settings = await getSettings();
      if (profile && settings) {
        if (!isAppInstalled) {
          appInstalled();
        }

        SplashScreen.hide();
        setOnboardingStatus('loggedIn');
        if (!currencies || !currencies.length) {
          updateAcceptedCoins({ coins: Object.keys(COINS) });
        }
        initializeLogin();

        if (!guest) {
          setGuest({
            name: profile.name,
            country: settings.country,
            localCurrency: settings.localCurrency,
          });
        }
        return;
      }

      SplashScreen.hide();

      if (guest) {
        const { country, localCurrency, name } = guest;
        const settings = await createSettings(country, localCurrency);
        setSettings(settings);
        const profile = await createProfile(name || 'Anonymous');
        setProfile(profile);
        setOnboardingStatus('loggedIn');
        updateAcceptedCoins({ coins: Object.keys(COINS) });
        console.warn('initialize called from guest');
        initializeLogin();
      } else {
        setOnboardingStatus('onboarding');
      }
    } catch (err) {
      // TODO: handle errors and show them in the UI. Currently an error in loading the settings
      // data results in a red screen of death (test by making the api/settings url invalid).
      // console.error(err);
      // console.log('initial onboarding call failure');
    }
  };

  render() {
    const { status, guest } = this.props;
    if (status === 'loading') {
      return <Text />;
    } else if ((status === 'offline' && guest) || status === 'loggedIn') {
      return <AppNavigator />;
    } else if ((status === 'offline' && !guest) || status === 'onboarding') {
      return <OnboardingContainer />;
    } else {
      return <Text>{I18n.t('OnboardingWrapper.unknown')}</Text>;
    }
  }
}

const mapStateToProps = state => ({
  status: state.appstate.onboardingStatus,
  isTrackingEvent: state.appstate.isTrackingEvent,
  guest: state.appstate.guest,
  username: state.appstate.username,
  password: state.appstate.password,
  isAppInstalled: state.appstate.appInstalled,
  currencies: state.profile.data.currencies,
});

const mapDispatchToProps = {
  setProfile,
  updateAcceptedCoins,
  setSettings,
  setOnboardingStatus,
  initializeLogin,
  triggerReviewPrompt,
  appInstalled,
  setGuest,
};

export default connect(mapStateToProps, mapDispatchToProps)(OnboardingWrapper);
