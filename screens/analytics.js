import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { ScrollView, View, Text } from 'react-native';

import Header from '../components/molecules/Header';
import InputGroup from '../components/atoms/InputGroup';
import SwitchInput from '../components/atoms/SwitchInput';
import DescriptionText from '../components/atoms/DescriptionText';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';
import { eventTracker } from '../utils/EventTracker';

import { setTrackingStatus } from '../reducers/appstate';
import { formLabelColor, primaryTextColor } from '../components/commonColors';

import {I18n} from '../langs/I18n';

const styles = {
  text: {
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  numbering: {
    marginVertical: 8,
    fontSize: 15,
    color: formLabelColor,
    lineHeight: 26,
  },
  description: {
    flex: 1,
    marginVertical: 8,
    color: formLabelColor,
  },
  textColor: { color: formLabelColor },
  primaryText: { color: primaryTextColor },
};

const shareDetails = [
  'Session information, such as how often you use the App and for how long.',
  'Basic device information; e.g., which type of phone you are using.',
  'The country you are accessing the App from.',
  'Which version of the App you are using.',
  'Which language you have selected.',
  'When you enter checkout for a purchase (no information is collected about what is being purchased).',
  'When you send funds, and which type of payment is used (no details are collected about the payment itself such as addresses or values).',
  'When you create a listing (no information about the listing itself is collected).',
  'Actions taken within Haven, such as tapping on the social feed or how often you make new posts. The content of the actions themselves are never recorded, only the fact that you took the action.',
];

class Analytics extends PureComponent {
  handleGoBack = () => {
    this.props.navigation.goBack();
  }

  handleToggle = (value) => {
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

  renderDetails = (text, idx) => (
    <View style={styles.text} key={`description_${idx}`}>
      <Text style={styles.numbering}>{idx + 1}. </Text>
      <DescriptionText key={`description_${idx}`} style={styles.description}>
        {text}
      </DescriptionText>
    </View>
  )

  render() {
    const { isTrackingEvent } = this.props;
    return (
      <View style={screenWrapper.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <ScrollView>
          <InputGroup title="Analytics">
            <SwitchInput
              useNative
              noBorder
              title="Share anonymous analytics"
              value={isTrackingEvent}
              onChange={this.handleToggle}
              style={styles.primaryText}
            />
            <DescriptionText style={styles.textColor}>
              If you opt into sharing analytics, you agree to share the following information with the OB1 Company:
            </DescriptionText>
            {shareDetails.map(this.renderDetails)}
          </InputGroup>
        </ScrollView>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  isTrackingEvent: state.appstate.isTrackingEvent,
  username: state.appstate.username,
  password: state.appstate.password,
});

const mapDispatchToProps = { setTrackingStatus };

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Analytics);
