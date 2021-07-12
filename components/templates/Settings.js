import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { ScrollView, Text, View, Alert } from 'react-native';
import * as _ from 'lodash';

import InputGroup from '../atoms/InputGroup';
import OptionGroup from '../atoms/OptionGroup';
import FormLabelText from '../atoms/FormLabelText';
import RadioModalFilter from '../molecules/RadioModalFilter';

import countries from '../../config/countries.json';
import currencies from '../../config/localCurrencies.json';

import { patchSettingsRequest } from '../../reducers/settings';
import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

countries.splice(0, 1);

const styles = {
  notificationLine: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  notificationLabel: {
    marginRight: 10,
  },
  scrollView: {
    paddingBottom: 12,
  },
  formLabel: {
    width: 150,
  },
  formValue: {
    fontSize: 14,
    color: primaryTextColor,
  },
  optionWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
};

class Settings extends PureComponent {
  componentDidMount() {
    const { settings } = this.props;
    const { localCurrency } = settings;
    if (localCurrency === 'VEF') {
      this.handleCurrencyChange('VES');
    }
  }

  getCurrencyLabel = option => option && `${option.name} (${option.symbol})`

  handleOpenResync = () => {
    this.props.navigation.navigate('Resync');
  };

  handleOpenCoinsAccepted = () => {
    this.props.navigation.navigate('AcceptedCoins');
  };

  handleOpenServerLog = () => {
    this.props.navigation.navigate('ServerLog');
  };

  handleGoToNotificationSettings = () => {
    this.props.navigation.navigate('NotificationSettings');
  };

  handleCountryChange = (country) => {
    this.props.patchSettingsRequest({ country: country.toUpperCase() });
  };

  handleCurrencyChange = (currency) => {
    this.props.patchSettingsRequest({ localCurrency: currency });
  };

  handleBackupProfile = () => {
    this.props.navigation.navigate('BackupProfileInit');
  }

  handleConfirmBeforeRestore = () => {
    Alert.alert(
      I18n.t('components.templates.Settings.are_you_sure'), 
      I18n.t('components.templates.Settings.check_backup'), 
      [
        {
          text: I18n.t('components.templates.Settings.cancel'),
          onPress: () => {},
          style: 'cancel',
        },
        { text: I18n.t('components.templates.Settings.OK'), onPress: this.handleRestoreProfile },
      ],
      { cancelable: false },
    );
  }


  handleRestoreProfile = () => {
    this.props.navigation.navigate('RestoreProfileInit');
  }

  render() {
    const {
      toShippingAddress,
      toPolicies,
      toModerators,
      toBlockedNodes,
      toBackupWallet,
      toAnalytics,
      settings,
      isTrackingEvent,
      currencies: acceptedCoins,
    } = this.props;
    const { country, localCurrency } = settings;

    return (
      <ScrollView contentContainerStyle={styles.scrollView}>
        <InputGroup title= {I18n.t('components.templates.Settings.profile')} >
          <RadioModalFilter
            title= {I18n.t('components.templates.Settings.Country')}
            secondary
            selected={_.isEmpty(country) ? '' : country.toLowerCase()}
            onChange={this.handleCountryChange}
            options={countries}
          />
          <RadioModalFilter
            title= {I18n.t('components.templates.Settings.currency')}
            secondary
            selected={_.isEmpty(localCurrency) ? '' : localCurrency === 'VEF' ? 'VES' : localCurrency}
            onChange={this.handleCurrencyChange}
            options={currencies}
            valueKey="code"
            getLabel={this.getCurrencyLabel}
          />
          <OptionGroup onPress={toShippingAddress} smallPadding>
            <FormLabelText text={I18n.t('components.templates.Settings.shipping_address')}/>
          </OptionGroup>
          <OptionGroup onPress={toBlockedNodes} noBorder smallPadding>
            <FormLabelText text= {I18n.t('components.templates.Settings.blocked')}/>
          </OptionGroup>
        </InputGroup>
        <InputGroup title= {I18n.t('components.templates.Settings.notifications')} >
          <OptionGroup onPress={this.handleGoToNotificationSettings} noBorder smallPadding>
            <FormLabelText text= {I18n.t('components.templates.Settings.push_notifications')}/> 
          </OptionGroup>
        </InputGroup>
        <InputGroup title= {I18n.t('components.templates.Settings.store')}>
          <OptionGroup onPress={toPolicies} smallPadding>
            <FormLabelText text= {I18n.t('components.templates.Settings.Policies')} />
          </OptionGroup>
          <OptionGroup onPress={toModerators} smallPadding>
            <FormLabelText text={I18n.t('components.templates.Settings.Moderators')} />
          </OptionGroup>
          <OptionGroup onPress={this.handleOpenCoinsAccepted} noBorder smallPadding>
            <FormLabelText text={I18n.t('components.templates.Settings.coins_accepted')} value={acceptedCoins.length > 0 && `${acceptedCoins.length} ${I18n.t('components.templates.Settings.selected')}`} />
          </OptionGroup>
        </InputGroup>
        <InputGroup title={I18n.t('components.templates.Settings.Advanced')}>
          <OptionGroup onPress={toAnalytics} smallPadding>
            <View style={styles.optionWrapper}>
              <FormLabelText text={I18n.t('components.templates.Settings.Analytics')} style={styles.formLabel} />
              <Text style={styles.formValue}>
                {isTrackingEvent ? I18n.t('components.templates.Settings.On') : I18n.t('components.templates.Settings.Off')}
              </Text>
            </View>
          </OptionGroup>
          <OptionGroup onPress={toBackupWallet} smallPadding>
            <FormLabelText text={I18n.t('components.templates.Settings.Backup_wallet')}  />
          </OptionGroup>
          <OptionGroup onPress={this.handleBackupProfile} smallPadding>
            <FormLabelText text= {I18n.t('components.templates.Settings.Backup_profile')} />
          </OptionGroup>
          <OptionGroup onPress={this.handleConfirmBeforeRestore} smallPadding>
            <FormLabelText text= {I18n.t('components.templates.Settings.Restore_profile')} />
          </OptionGroup>
          <OptionGroup onPress={this.handleOpenResync} smallPadding>
            <FormLabelText text= {I18n.t('components.templates.Settings.Resync_transactions')} />
          </OptionGroup>
          <OptionGroup onPress={this.handleOpenServerLog} smallPadding>
            <FormLabelText text= {I18n.t('components.templates.Settings.Server_Log')} />
          </OptionGroup>
          <FormLabelText text= {I18n.t('components.templates.Settings.Version')} />
        </InputGroup>
      </ScrollView>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
  isTrackingEvent: state.appstate.isTrackingEvent,
  profile: state.profile.data,
  settings: state.settings,
  unreadCount: state.notifications.unread,
  currencies: state.profile.data.currencies,
});

const mapDispatchToProps = {
  patchSettingsRequest,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Settings);
