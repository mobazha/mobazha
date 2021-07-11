import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';

import { fetchProfile } from '../reducers/profile';
import { fetchSettings } from '../reducers/settings';
import { fetchModerationSettings } from '../reducers/moderationSettings';
import { screenWrapper } from '../utils/styles';

import Header from '../components/molecules/Header';
import SettingsTemplate from '../components/templates/Settings';
import BackupWallet from '../components/templates/BackupWallet';
import NavBackButton from '../components/atoms/NavBackButton';

import { copiedBackupPhrase } from '../reducers/appstate';

import {I18n} from '../langs/I18n';
class Settings extends PureComponent {
  state = { showBackup: false };

  UNSAFE_componentWillMount() {
    this.props.fetchSettings();
  }

  handleNavigation = route => () => {
    this.props.navigation.navigate(route);
  };

  handleShowBackup = () => {
    this.setState({ showBackup: true });
  };

  handleCloseBackup = () => {
    this.setState({ showBackup: false });
  };

  toModerationSettings = () => {
    this.props.fetchModerationSettings();
    this.props.navigation.navigate('ModerationSettings');
  };

  render() {
    const { navigation, copiedBackupPhrase } = this.props;
    const { showBackup } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => navigation.goBack(null)}
          title={I18n.t('screens.settings.Settings')}
        />
        <SettingsTemplate
          toShippingAddress={this.handleNavigation('ShippingAddress')}
          toPolicies={this.handleNavigation('Policies')}
          toModerators={this.handleNavigation('StoreModerators')}
          toModerationSettings={this.toModerationSettings}
          toBlockedNodes={this.handleNavigation('BlockedNodes')}
          toBackupWallet={this.handleShowBackup}
          toAnalytics={this.handleNavigation('Analytics')}
          navigation={navigation}
        />
        <BackupWallet
          show={showBackup}
          onClose={this.handleCloseBackup}
          onFinishBackup={copiedBackupPhrase}
        />
      </View>
    );
  }
}

const mapDispatchToProps = {
  fetchProfile,
  fetchSettings,
  fetchModerationSettings,
  copiedBackupPhrase,
};

export default connect(
  null,
  mapDispatchToProps,
)(Settings);
