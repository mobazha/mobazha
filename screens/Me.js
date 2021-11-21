import React, { PureComponent } from 'react';
import { View, Text, FlatList, ScrollView, Linking } from 'react-native';
import { connect } from 'react-redux';
import * as _ from 'lodash';
import Feather from 'react-native-vector-icons/Feather';
import MaterialIcons from 'react-native-vector-icons/MaterialIcons';
import decode from 'unescape';

import MeActionItem from '../components/atoms/MeActionItem';
import TabHeader from '../components/organism/TabHeader';

import { foregroundColor, primaryTextColor, greenColor, borderColor } from '../components/commonColors';
import OptionGroup from '../components/atoms/OptionGroup';
import Button from '../components/atoms/FullButton';
import NavCloseButton from '../components/atoms/NavCloseButton';
import Header from '../components/molecules/Header';
import InputGroup from '../components/atoms/InputGroup';
import DescriptionText from '../components/atoms/DescriptionText';
import AvatarImage from '../components/atoms/AvatarImage';
import LocationPin from '../components/atoms/LocationPin';
import { OBLightModal } from '../components/templates/OBModal';
import { eventTracker } from '../utils/EventTracker';
import { FAQ_URL, DISCORD_URL, EMAIL_URL, TELEGRAM_URL } from '../config/supportUrls';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    alignItems: 'center',
  },
  optionContent: {
    paddingLeft: 20,
  },
  optionWrapper: {
    flexDirection: 'row',
    paddingVertical: 10,
  },
  imageWrapper: {
    width: 60,
    height: 60,
    marginRight: 15,
  },
  actionWrapper: {
    flex: 1,
    justifyContent: 'flex-end',
    paddingBottom: 17.5,
  },
  infoContainer: {
    flexDirection: 'column',
    justifyContent: 'space-around',
  },
  text: {
    color: primaryTextColor,
    fontSize: 16,
    fontWeight: 'bold',
    paddingTop: 10,
    paddingBottom: 2.5,
  },
  contentWrapper: {
    flex: 1,
  },
  buttonWrapper: {
    backgroundColor: foregroundColor,
    borderWidth: 1,
    borderColor,
    marginTop: 0,
    marginBottom: 10,
  },
  lastButton: {
    marginBottom: 25,
  },
  buttonText: {
    color: primaryTextColor,
  },
  bold: {
    fontWeight: 'bold',
  },
  firstButton: {
    marginBottom: 10,
  },
};

const actions = [
  {
    caption: I18n.t('screens.Me.my_Profile'),
    icon: props => <Feather name="user" color={greenColor} size={24} {...props} />,
    screenName: 'Store',
  },
  {
    caption: I18n.t('screens.Me.wallet'),
    icon: props => <MaterialIcons name="account-balance-wallet" color={greenColor} size={24} {...props} />,
    screenName: 'Wallet',
  },
  {
    caption:  I18n.t('screens.Me.purchases'),
    icon: props => <Feather name="shopping-cart" color={greenColor} size={24} {...props} />,
    screenName: 'Orders',
    params: { orderType: 'purchases' },
  },
  {
    caption: I18n.t('screens.Me.sales'),
    icon: props => <Feather name="tag" color={greenColor} size={24} {...props} />,
    screenName: 'Orders',
    params: { orderType: 'sales' },
  },
  {
    caption: I18n.t('screens.Me.wishlist'),
    icon: props => <Feather name="heart" color={greenColor} size={24} {...props} />,
    screenName: 'WishList',
  },
  {
    caption: I18n.t('screens.Me.settings'),
    icon: props => <Feather name="settings" color={greenColor} size={24} {...props} />,
    screenName: 'Settings',
  },
  {
    caption: I18n.t('screens.Me.notifications'),
    icon: props => <Feather name="bell" color={greenColor} size={24} {...props} />,
    screenName: 'Notifications',
    isNotif: true,
  },
  {
    caption: I18n.t('screens.Me.support'),
    icon: props => <Feather name="info" color={greenColor} size={24} {...props} />,
    screenName: 'Support',
  },
];

class Me extends PureComponent {
  state = { showSupport: false };

  onItemPress = (screenName, params) => () => {
    eventTracker.trackEvent('Me-TappedNavigation', { screenName });
    if (screenName === 'Support') {
      this.setState({ showSupport: true });
    } else {
      this.props.navigation.navigate(screenName, params);
    }
  }

  handlePressFaq = () => {
    Linking.openURL(FAQ_URL);
  }

  handlePressDiscord = () => {
    Linking.openURL(DISCORD_URL);
  }

  handlePressTelegram = () => {
    Linking.openURL(TELEGRAM_URL);
  }

  handlePressEmailSupport = () => {
    Linking.openURL(EMAIL_URL);
  }

  handleHideModal = () => this.setState({ showSupport: false })

  handleGoToMyStore = () => this.props.navigation.navigate('Store')

  keyExtractor = item => item.caption;

  renderItem = ({ item }) => {
    const { screenName, params, ...details } = item;
    return (
      <MeActionItem item={details} onPress={this.onItemPress(screenName, params)} />
    );
  }

  render() {
    const { name = '', avatarHashes, location } = this.props.profile;
    const { showSupport } = this.state;
    return (
      <View style={styles.wrapper}>
        <TabHeader title={I18n.t('screens.Me.me')} />
        <OptionGroup contentStyle={styles.optionContent} onPress={this.handleGoToMyStore}>
          <View style={styles.optionWrapper}>
            <AvatarImage style={styles.imageWrapper} thumbnail={_.get(avatarHashes, 'tiny')} showLocal />
            <View style={styles.infoContainer}>
              <Text style={styles.text} numberOfLines={1}>{decode(name)}</Text>
              <LocationPin location={decode(location)} />
            </View>
          </View>
        </OptionGroup>
        <FlatList
          style={{ flex: 1 }}
          contentContainerStyle={styles.actionWrapper}
          data={actions}
          renderItem={this.renderItem}
          keyExtractor={this.keyExtractor}
          numColumns={4}
        />
        <OBLightModal
          animationType="slide"
          transparent
          visible={showSupport}
          onRequestClose={this.handleHideModal}
        >
          <Header
            left={<NavCloseButton />}
            modal
            onLeft={this.handleHideModal}
          />
          <ScrollView style={styles.contentWrapper} contentContainerStyle={styles.contentWrapper}>
            <InputGroup title={I18n.t('screens.Me.support2')} noBorder>
              <DescriptionText>
              {I18n.t('screens.Me.Description1')} 
              {I18n.t('screens.Me.Description2')}  <Text style={styles.bold}>  {I18n.t('screens.Me.Description3')}  </Text>  {I18n.t('screens.Me.Description4')} 
              </DescriptionText>
              <DescriptionText>
              {I18n.t('screens.Me.Description5')}
              </DescriptionText>
            </InputGroup>
          </ScrollView>
          <Button
            title={I18n.t('screens.Me.discord')}
            wrapperStyle={styles.buttonWrapper}
            textStyle={styles.buttonText}
            onPress={this.handlePressDiscord}
            style={styles.firstButton}
          />
          <Button
            title={I18n.t('screens.Me.telegram')}
            textStyle={styles.buttonText}
            wrapperStyle={styles.buttonWrapper}
            onPress={this.handlePressTelegram}
          />
          <Button
            title={I18n.t('screens.Me.email_Support')}
            textStyle={styles.buttonText}
            wrapperStyle={[styles.buttonWrapper, styles.lastButton]}
            onPress={this.handlePressEmailSupport}
          />
        </OBLightModal>
      </View>
    );
  }
}

const mapStateToProps = state => ({ profile: state.profile.data || {} });

export default connect(mapStateToProps)(Me);
