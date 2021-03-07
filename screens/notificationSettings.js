import React, { Fragment, PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, ScrollView } from 'react-native';

import Header from '../components/molecules/Header';
import InputGroup from '../components/atoms/InputGroup';
import SwitchInput from '../components/atoms/SwitchInput';
import DescriptionText from '../components/atoms/DescriptionText';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';

import { updateNotificationSettings } from '../reducers/appstate';
import { formLabelColor, primaryTextColor } from '../components/commonColors';

import {I18n} from '../langs/I18n';

const styles = {
  content: {
    paddingTop: 24,
  },
  description: {
    fontSize: 13,
    lineHeight: 17,
    color: formLabelColor,
    marginVertical: 0,
    paddingBottom: 24,
    paddingRight: 60,
  },
  switchText: {
    color: primaryTextColor,
    fontWeight: '600',
    paddingBottom: 4,
  },
  switchWrapper: {
    paddingVertical: 0,
  },
};

class NotificationSettings extends PureComponent {
  handleGoBack = () => {
    this.props.navigation.goBack();
  };

  renderItem = (title, description, valueKey) => {
    const { [valueKey]: value, updateNotificationSettings } = this.props;
    return (
      <Fragment>
        <SwitchInput
          noBorder
          title={title}
          value={value}
          onChange={value => updateNotificationSettings({ topic: valueKey, enable: value })}
          style={styles.switchText}
          wrapperStyle={styles.switchWrapper}
          useNative
        />
        <DescriptionText style={styles.description}>
          {description}
        </DescriptionText>
      </Fragment>
    );
  };

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <ScrollView>
          <InputGroup title={I18n.t('screens.notificationSettings.notification_preferences')} noBorder contentStyle={styles.content}>
            {this.renderItem(
              I18n.t('screens.notificationSettings.all1'),
              I18n.t('screens.notificationSettings.Receive_all'),
              'all',
            )}
            {this.renderItem(
              I18n.t('screens.notificationSettings.featured_content'),
              I18n.t('screens.notificationSettings.notify1'),
              'promotions',
            )}
            {this.renderItem(
              I18n.t('screens.notificationSettings.giveaways1'),
              I18n.t('screens.notificationSettings.Notify2'),
              'giveaways',
            )}
            {this.renderItem(
              I18n.t('screens.notificationSettings.announcements1'),
              I18n.t('screens.notificationSettings.notify3'),
              'announcements',
            )}
            {this.renderItem(
              I18n.t('screens.notificationSettings.chat1'),
              I18n.t('screens.notificationSettings.notify4'),
              'chat',
            )}
            {/* {this.renderItem(
              'Orders',
              'Notify me when there\'s an update to one of my orders.',
              'orders',
            )} */}
            {this.renderItem(
              I18n.t('screens.notificationSettings.likes1'),
              I18n.t('screens.notificationSettings.notify5'),
              'likes',
            )}
            {this.renderItem(
              I18n.t('screens.notificationSettings.comments1'),
              I18n.t('screens.notificationSettings.notify6'),
              'comments',
            )}
          </InputGroup>
        </ScrollView>
      </View>
    );
  }
}

const mapStateToProps = (state) => {
  // const {
  //   promotions, announcements, giveaways, chat, orders, likes, comments,
  // } = state.appstate.notifications;
  // const all = promotions && announcements && giveaways && chat && orders && likes && comments;
  // return {
  //   promotions, announcements, giveaways, chat, orders, likes, comments, all,
  // };
  return {};
};

const mapDispatchToProps = {
  updateNotificationSettings,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(NotificationSettings);
