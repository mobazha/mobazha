import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Keyboard, Platform, View, Image, Switch, Text, TouchableWithoutFeedback } from 'react-native';
import * as _ from 'lodash';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';

import InputGroup from '../components/atoms/InputGroup';
import TextInput from '../components/atoms/TextInput';
import SelectorModal from '../components/organism/SelectorModal';
import { primaryTextColor, formLabelColor, borderColor } from '../components/commonColors';
import Button from '../components/atoms/FullButton';
import { screenWrapper, onboardingStyles, footerStyles } from '../utils/styles';

import countries from '../config/countries.json';
import currencies from '../config/localCurrencies.json';
import { setOnboardingStatus, setTrackingStatus } from '../reducers/appstate';
import { StatusBarSpacer } from '../status-bar';

import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n'

const logoImg = require('../assets/images/logo/brand-logo.png');
const onboardingImg = require('../assets/images/onboardingBackground.png');

const titleWidth = 100;

const styles = {
  img: {
    position: 'absolute',
    left: 0,
    top: 0,
    width: '100%',
    height: '100%',
  },
  logo: {
    width: 66,
    height: 32,
  },
  helloText: {
    fontSize: 13,
    lineHeight: 13,
    color: 'white',
    backgroundColor: '#8cd885',
    left: 16,
    paddingLeft: 11,
    paddingRight: 9,
    paddingTop: 7,
    paddingBottom: 5,
    letterSpacing: 1,
    borderRadius: 13,
    overflow: 'hidden',
    alignSelf: 'flex-start',
    marginTop: -13,
  },
  nameContainer: {
    marginTop: -4,
  },
  selectorContainer: {
    marginTop: 16,
  },
  analyticsContainer: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: 16,
  },
  analyticsText: {
    color: formLabelColor,
  },
  analyticsTitle: {
    color: primaryTextColor,
    marginBottom: 6,
  },
  restoreContainer: {
    marginTop: -13,
    alignItems: 'flex-end',
  },
  restore: {
    marginRight: 16,
    paddingVertical: 6,
    paddingHorizontal: 8,
    textAlign: 'center',
    fontSize: 14,
    fontWeight: 'bold',
    borderWidth: 1,
    borderRadius: 4,
    borderColor,
    color: primaryTextColor,
  },
};

const initialState = {
  name: '',
  country: {
    value: 'united_states',
    label: '🇺🇸 United States of America',
    checked: false,
    default: false,
  },
  currency: {
    symbol: '$',
    name: 'US Dollar',
    symbol_native: '$',
    decimal_digits: 2,
    rounding: 0,
    code: 'USD',
    name_plural: 'US dollars',
  },
  loggingIn: false,
};

class Onboarding extends PureComponent {
  state = initialState;

  componentDidMount() {
    if (Platform.OS === 'ios') {
      this.keyboardWillShowSub = Keyboard.addListener('keyboardWillShow', this.keyboardWillShow);
      this.keyboardWillHideSub = Keyboard.addListener('keyboardWillHide', this.keyboardWillHide);
    } else {
      this.keyboardWillShowSub = Keyboard.addListener('keyboardDidShow', this.keyboardWillShow);
      this.keyboardWillHideSub = Keyboard.addListener('keyboardDidHide', this.keyboardWillHide);
    }
  }

  componentWillUnmount() {
    this.keyboardWillShowSub.remove();
    this.keyboardWillHideSub.remove();
  }

  onChangeCountry = (country) => {
    this.setState({ country });
    eventTracker.trackEvent('Onboarding-ChangedCountry', { country });
  }

  onChangeCurrency = (currency) => {
    this.setState({ currency });
    eventTracker.trackEvent('Onboarding-ChangedCurrency', { currency });
  }

  onChangeName = (name) => {
    this.setState({ name });
    eventTracker.trackEvent('Onboarding-FilledOutProfile');
  }

  keyboardWillShow = () => {
    this.setState({ keyboardVisible: true });
  };

  keyboardWillHide = () => {
    this.setState({ keyboardVisible: false });
  };

  handleRestore = async () => {
    const { navigation } = this.props;
    navigation.navigate('RestoreProfileInit');
  };

  handleGetStarted = async () => {
    const { navigation } = this.props;
    const { name, country, currency } = this.state;
    navigation.navigate('Privacy', { params: { name, country, currency } });
  };

  handleToggleAnalytics = (value) => {
    // If tracking was on, the event must be fired before it is turned back off.
    if (!value) {
      eventTracker.trackEvent('Onboarding-OptedIntoAnalytics', { value: value.toString() });
    }

    this.props.setTrackingStatus(value);

    // If tracking was off, the event must be fired after it is turned back on.
    if (value) {
      eventTracker.trackEvent('Onboarding-OptedIntoAnalytics', { value: value.toString() });
    }
  }

  render() {
    const {
      name, country, currency, loggingIn, keyboardVisible,
    } = this.state;
    const { isTrackingEvent } = this.props;

    return (
      <KeyboardAwareScrollView
        style={screenWrapper.wrapper}
        contentContainerStyle={!keyboardVisible && { flex: 1 }}
        keyboardShouldPersistTaps="handled"
      >
        <View style={onboardingStyles.header}>
          <Image style={styles.img} source={onboardingImg} resizeMode="stretch" />
          <View style={onboardingStyles.headerContent}>
            <StatusBarSpacer />
            <Image style={styles.logo} source={logoImg} resizeMode="contain" />
          </View>
        </View>
        <Text style={styles.helloText}>{I18n.t('screens.onboarding.HELLO')}</Text>
        <TouchableWithoutFeedback onPress={this.handleRestore} disabled={loggingIn}>
          <View style={styles.restoreContainer}>
            <Text style={styles.restore}>
            {I18n.t('screens.onboarding.restore_profile')}
            </Text>
          </View>
        </TouchableWithoutFeedback>
        <InputGroup noBorder noHeaderPadding noHeaderTopPadding>
          <TextInput
            wrapperStyle={styles.nameContainer}
            title={I18n.t('screens.onboarding.name')}
            value={name}
            onChangeText={this.onChangeName}
            placeholder={I18n.t('screens.onboarding.optional')}
            titleWidth={titleWidth}
            vertical
          />
          <SelectorModal
            wrapperStyle={styles.selectorContainer}
            title={I18n.t('screens.onboarding.country')}
            value={country}
            onChange={this.onChangeCountry}
            options={countries}
            titleWidth={titleWidth}
            vertical
          />
          <SelectorModal
            wrapperStyle={styles.selectorContainer}
            title={I18n.t('screens.onboarding.currency')}
            value={currency}
            noBorder
            onChange={this.onChangeCurrency}
            options={currencies}
            valueKey="code"
            getLabel={option => `${option.name} (${option.symbol})`}
            titleWidth={titleWidth}
            vertical
          />
          <View style={styles.analyticsContainer}>
            <View>
              <Text style={styles.analyticsTitle}>{I18n.t('screens.onboarding.share_analytics')}</Text>
              <Text style={styles.analyticsText}>{I18n.t('screens.onboarding.help_improve')}</Text>
            </View>
            <Switch
              onValueChange={this.handleToggleAnalytics}
              value={isTrackingEvent}
            />
          </View>
        </InputGroup>
        {!keyboardVisible && <View style={{ flex: 1 }} />}
        <View style={footerStyles.roundButtonContainer}>
          <Button
            wrapperStyle={onboardingStyles.button}
            title={I18n.t('screens.onboarding.HELLO')}
            onPress={this.handleGetStarted}
            disabled={loggingIn}
          />
        </View>
      </KeyboardAwareScrollView>
    );
  }
}

const mapStateToProps = state => ({
  isTrackingEvent: state.appstate.isTrackingEvent,
});

const mapDispatchToProps = {
  setOnboardingStatus,
  setTrackingStatus,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Onboarding);
