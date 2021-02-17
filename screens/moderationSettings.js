import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert } from 'react-native';
import { isEmpty } from 'lodash';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import ModerationSettingsTemplate from '../components/templates/ModerataionSettings';
import { screenWrapper } from '../utils/styles';

import { updateModerationSettings } from '../reducers/moderationSettings';
import NavBackButton from '../components/atoms/NavBackButton';

import {I18n} from '../langs/I18n';
class ModerationSettings extends PureComponent {
  state = {
    description: '',
    termsAndConditions: '',
    languages: [],
    fee: {},
  };

  validateFields() {
    const {
      description, termsAndConditions, languages, fee,
    } = this.state;
    if (isEmpty(description)) {
      Alert.alert('Description is required');
      return;
    }
    if (isEmpty(termsAndConditions)) {
      Alert.alert('Terms of service is required');
      return;
    }
    if (isEmpty(languages)) {
      Alert.alert('At least one language is required');
      return;
    }
    switch (fee.feeType) {
      case 'FIXED':
        if (isEmpty(fee.fixedFee.amount)) {
          Alert.alert('Fee is required');
          return;
        }
        break;
      case 'PERCENTAGE':
        if (isEmpty(fee.percentage)) {
          Alert.alert('Percentage is required');
          return;
        }
        if (fee.percentage <= 0 || fee.percentage >= 100) {
          Alert.alert('Percentage is invalid');
          return;
        }
        break;
      default:
        if (isEmpty(fee.fixedFee.amount)) {
          Alert.alert('Fee is required');
          return;
        }
        if (isEmpty(fee.percentage)) {
          Alert.alert('Percentage is required');
          return;
        }
        if (fee.percentage <= 0 || fee.percentage >= 100) {
          Alert.alert('Percentage is invalid');
          return;
        }
        break;
    }
    this.props.updateModerationSettings(this.state);
  }

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          title="Moderator Settings"
          right={<LinkText text="Done" />}
          onRight={() => {
            this.validateFields();
            this.props.navigation.navigate('MainSettings');
          }}
        />
        <ModerationSettingsTemplate
          onChange={(val) => {
            this.setState({ ...val });
          }}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({});

const mapDispatchToProps = {
  updateModerationSettings,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ModerationSettings);
