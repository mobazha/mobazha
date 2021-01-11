import React from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback, ScrollView, Alert, Share } from 'react-native';
import Octicons from 'react-native-vector-icons/Octicons';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { get, filter } from 'lodash';
import * as striptags from 'striptags';
import he from 'he';
import decode from 'unescape';

import Avatar from '../components/atoms/AvatarImage';
import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import NavOptionButton from '../components/atoms/NavOptionButton';
import Button from '../components/atoms/FullButton';
import InputGroup from '../components/atoms/InputGroup';

import { primaryTextColor, foregroundColor, staticLabelColor, borderColor, brandColor, formLabelColor } from '../components/commonColors';
import { patchSettingsRequest, blockNode } from '../reducers/settings';
import { setModerator } from '../reducers/appstate';
import { convertorsMap } from '../selectors/currency';
import Fee from '../components/atoms/Fee';
import LocationPin from '../components/atoms/LocationPin';

import { eatSpaces } from '../utils/string';
import OBActionSheet from '../components/organism/ActionSheet';
import { eventTracker } from '../utils/EventTracker';
import { createStoreUrlFromPeerID } from '../utils/navigation';

import {I18n} from '../langs/I18n';

const styles = {
  profileWrapper: {
    backgroundColor: '#ffffff',
    paddingHorizontal: 22,
    paddingVertical: 17,
    borderBottomWidth: 1,
    borderColor,
  },
  avatar: {
    height: 50,
    width: 50,
  },
  rowContainer: {
    marginTop: 14,
    flexDirection: 'row',
    alignItems: 'center',
  },
  name: {
    color: primaryTextColor,
    fontSize: 16,
    fontWeight: 'bold',
    marginRight: 7,
  },
  verified: {
    marginLeft: 3,
    fontSize: 13,
    fontWeight: 'bold',
    color: brandColor,
  },
  location: {
    marginTop: 16,
    fontSize: 15,
    color: primaryTextColor,
  },
  feeContainer: {
    marginTop: 10,
  },
  fee: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    color: primaryTextColor,
  },
  feeDescription: {
    fontSize: 13,
    fontWeight: 'normal',
    letterSpacing: 0,
    color: formLabelColor,
  },
  description: {
    marginTop: 14,
  },
  descriptionText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    color: primaryTextColor,
  },
  details: {
    backgroundColor: foregroundColor,
    borderBottomWidth: 1,
    borderTopWidth: 1,
    borderColor,
    paddingHorizontal: 20,
    paddingVertical: 15,
  },
  topSection: {
    paddingBottom: 20,
    borderStyle: 'solid',
    borderBottomWidth: 1,
    borderColor,
  },
  sectionTitle: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: staticLabelColor,
    marginBottom: 9,
  },
  languages: {
    marginBottom: 15,
  },
  language: {
    marginTop: 15,
    fontSize: 15,
    color: primaryTextColor,
  },
  termContent: {
    marginVertical: 15,
    fontSize: 15,
    color: primaryTextColor,
  },
  emptyTerm: {
    marginTop: 10,
  },
  bottomSection: {
    paddingTop: 20,
  },
  footer: {
    backgroundColor: 'white',
    flexDirection: 'row',
    justifyContent: 'flex-end',
    alignItems: 'center',
    borderTopWidth: 1,
    borderColor: '#e8e8e8',
    height: 76,
  },
  selectButton: {
    width: 120,
  },
  selected: {
    fontSize: 14,
    alignItems: 'center',
    color: brandColor,
    marginRight: 17,
  },
};

class ModeratorDetails extends React.PureComponent {
  state = {
    showTermContent: false,
  }

  getProfile() {
    const { profiles, navigation } = this.props;
    const moderator = navigation.getParam('moderator');
    return profiles[moderator];
  }

  isCheckoutFlow() {
    const { routeName } = this.props.navigation.state;
    return routeName === 'CheckoutModeratorDetails';
  }

  selectModerator = () => {
    const { setModerator, navigation } = this.props;
    const moderator = navigation.getParam('moderator');
    setModerator(moderator);
    eventTracker.trackEvent('Checkout-ChangedModerator');
    navigation.pop(2);
  };

  removeModerator = () => {
    const { storeModerators, patchSettingsRequest, navigation } = this.props;
    const moderator = navigation.getParam('moderator');
    const newModerators = filter(storeModerators, o => o !== moderator);
    Alert.alert(
      I18n.t('screens.moderatorDetails.remove_moderator'),
      I18n.t('screens.moderatorDetails.remove_hint'),
      [
        { text:I18n.t('screens.moderatorDetails.cancel') },
        {
          text: 'OK',
          onPress: () => {
            navigation.goBack();
            patchSettingsRequest({
              storeModerators: newModerators,
            });
          },
        },
      ],
    );
  };

  handleShowActionSheet = () => {
    this.ActionSheet.show();
  };

  handleActionSheet = (idx) => {
    const moderator = this.props.navigation.getParam('moderator');
    const { name } = this.getProfile();

    switch (idx) {
      case 0:
        if (this.isCheckoutFlow()) {
          this.selectModerator();
        } else {
          this.removeModerator();
        }
        break;
      case 1:
        this.props.navigation.navigate('ExternalStore', { peerID: moderator });
        break;
      case 2:
        Share.share({
          message: createStoreUrlFromPeerID(moderator),
          title: name,
        });
        break;
      default:
        break;
    }
  };

  handleToggleTerm = () => {
    const { showTermContent } = this.state;
    this.setState({ showTermContent: !showTermContent });
  };

  handleGoBack = () => {
    const { navigation } = this.props;
    navigation.goBack();
  };

  render() {
    const { navigation, moderator: selectedModerator } = this.props;
    const moderator = navigation.getParam('moderator');
    const profile = this.getProfile();
    const { peerID, name, location, moderatorInfo } = profile || {};
    const { description, languages, termsAndConditions, fee } = moderatorInfo || {};

    const avatarHash = get(profile, 'avatarHashes.tiny');

    const { showTermContent } = this.state;

    const actionSheetOptions = [
      this.isCheckoutFlow() ? 'Select moderator' : 'Remove moderator',
      'View profile',
      'Share to...',
      'Cancel',
    ];

    return (
      <View style={{ flex: 1 }}>
        <Header
          left={<NavBackButton />}
          onLeft={this.handleGoBack}
          right={<NavOptionButton />}
          onRight={this.handleShowActionSheet}
        />
        <ScrollView style={{ flex: 1, backgroundColor: 'white' }}>
          <TouchableWithoutFeedback
            onPress={() => {
              this.props.navigation.navigate({ routeName: 'ExternalStore', params: { peerID } });
            }}
          >
            <View style={styles.profileWrapper}>
              <Avatar thumbnail={avatarHash} style={styles.avatar} />
              <View style={styles.rowContainer}>
                <Text style={styles.name}>
                  {decode(name)}
                </Text>
                <Octicons name="verified" size={14} color={brandColor} />
                <Text style={styles.verified}>{I18n.t('screens.moderatorDetails.verified')}</Text>
              </View>
              <Text style={[styles.description, styles.descriptionText]}>
                {eatSpaces(he.decode(striptags.default(description)))}
              </Text>
              <LocationPin style={styles.location} location={decode(location)} />
              <View style={styles.feeContainer}>
                {fee && (
                  <Fee style={styles.fee} moderatorInfo={fee} verbose />
                )}
                <Text style={styles.feeDescription}>
                {I18n.t('screens.moderatorDetails.fee_description')}
                </Text>
              </View>
              <View style={[styles.rowContainer]}>
                <Octicons name="verified" size={14} color={brandColor} />
                <Text style={styles.descriptionText}>
                  {I18n.t('screens.moderatorDetails.moderator_verified')}
                </Text>
              </View>
            </View>
          </TouchableWithoutFeedback>
          {languages && (
            <InputGroup title="Languages">
              <View style={styles.languages}>
                {languages.map(lang => <Text style={styles.language}>{`${lang}`}</Text>)}
              </View>
            </InputGroup>
          )}
          <InputGroup
            title={I18n.t('screens.moderatorDetails.terms')}
            actionTitle={
              <Ionicons
                name={showTermContent ? 'ios-arrow-down' : 'ios-arrow-forward'}
                size={24}
                color={primaryTextColor}
              />
            }
            action={this.handleToggleTerm}
          >
            <Text style={showTermContent ? styles.termContent : styles.emptyTerm}>
              {showTermContent && eatSpaces(he.decode(striptags.default(termsAndConditions)))}
            </Text>
          </InputGroup>
        </ScrollView>
        {this.isCheckoutFlow() && (
          <View style={styles.footer}>
            {moderator === selectedModerator ? (
              <Text style={styles.selected}>
                <Ionicons name="md-checkmark" size={18} color={brandColor} />
                {I18n.t('screens.moderatorDetails.selected')}
              </Text>
            ) : (
              <Button wrapperStyle={styles.selectButton} title={I18n.t('screens.moderatorDetails.select')} onPress={this.selectModerator} />
            )}        
          </View>
        )}
        <OBActionSheet
          ref={(o) => {
            this.ActionSheet = o;
          }}
          options={actionSheetOptions}
          cancelButtonIndex={3}
          onPress={this.handleActionSheet}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  moderator: state.appstate.moderator,
  storeModerators: state.settings.storeModerators,
  profiles: state.profiles,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  patchSettingsRequest,
  blockNode,
  setModerator,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ModeratorDetails);
