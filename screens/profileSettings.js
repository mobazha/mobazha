import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, findNodeHandle, Alert, Keyboard } from 'react-native';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';
import * as _ from 'lodash';

import { fetchProfile, updateProfile } from '../reducers/profile';
import { screenWrapper } from '../utils/styles';

import ProfileImages from '../components/organism/ProfileImages';
import InputGroup from '../components/atoms/InputGroup';
import TextInput from '../components/atoms/TextInput';
import TextArea from '../components/atoms/TextArea';
import EditProfileBanner from '../components/atoms/EditProfileBanner';
import EditProfileHeader from '../components/molecules/EditProfileHeader';
import { StatusBarSpacer } from '../status-bar';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';
class ProfileSettings extends PureComponent {
  constructor(props) {
    super(props);
    this.state = { ...props.profile, keyboardVisible: false };
  }

  componentDidMount() {
    this.keyboardDidShowSub = Keyboard.addListener('keyboardDidShow', this.keyboardDidShow);
    this.keyboardWillHideSub = Keyboard.addListener('keyboardDidHide', this.keyboardWillHide);
  }

  componentDidUpdate(prevProps) {
    if (!_.isEqual(prevProps.profile, this.props.profile)) {
      this.setState({ ...prevProps.profile })
    }
  }

  onLeft = () => {
    Alert.alert(
      I18n.t('screens.profileSettings.warning'),
      I18n.t('screens.profileSettings.warning_info'),
      [
        { text: I18n.t('screens.profileSettings.Cancel')},
        { text: I18n.t('screens.profileSettings.OK'), onPress: () => { this.props.navigation.goBack(); } },
      ],
      { cancelable: false }
    );
  };

  onSuccess = () => {
    this.props.navigation.goBack();
  };

  keyboardDidShow = () => {
    this.setState({ keyboardVisible: true });
  };

  keyboardWillHide = () => {
    this.setState({ keyboardVisible: false });
  };

  findPosition = (event) => {
    this.scrollToInput(findNodeHandle(event.target));
  };

  handleChange = key => (value) => {
    if (key === 'imageHashes') {
      this.setState({ ...value });
    } else {
      this.setState({ [key]: value });
    }
  };

  handleContactChange = key => (value) => {
    const { contactInfo } = this.state;
    const newContactInfo = { ...contactInfo };
    newContactInfo[key] = value;
    this.setState({ contactInfo: newContactInfo });
  };

  handleSave = () => {
    const { keyboardVisible, ...profile } = this.state;
    eventTracker.trackEvent('EditProfile-UpdatedProfileInfo');
    this.props.updateProfile({ data: profile, onSuccess: this.onSuccess });
  };

  scrollToInput(reactNode) {
    this.scroll.props.scrollToFocusedInput(reactNode);
  }

  render() {
    const {
      name,
      shortDescription,
      location,
      contactInfo = {},
      about,
      avatarHashes,
      headerHashes,
      keyboardVisible,
    } = this.state;
    const { email = '', phoneNumber = '', website = '' } = contactInfo;
    return (
      <View style={screenWrapper.wrapper}>
        <EditProfileHeader onBack={this.onLeft} />
        <StatusBarSpacer />
        <KeyboardAwareScrollView
          innerRef={(ref) => {
            this.scroll = ref;
          }}
        >
          <ProfileImages
            avatarHashes={avatarHashes}
            headerHashes={headerHashes}
            onChange={this.handleChange('imageHashes')}
          />
          <View style={screenWrapper.wrapper}>
            <InputGroup title={I18n.t('screens.profileSettings.profile_information')} noHeaderTopPadding>
              <TextInput
                title={I18n.t('screens.profileSettings.name')}
                required
                value={name}
                onChangeText={this.handleChange('name')}
                onFocus={this.findPosition}
                placeholder={I18n.t('screens.profileSettings.name_hint')}
                maxLength={40}
              />
              <TextInput
                title={I18n.t('screens.profileSettings.bio')}
                value={shortDescription}
                onChangeText={this.handleChange('shortDescription')}
                onFocus={this.findPosition}
                placeholder={I18n.t('screens.profileSettings.bio_hint')}
                maxLength={140}
              />
              <TextInput
                noBorder
                title={I18n.t('screens.profileSettings.location')}
                value={location}
                onChangeText={this.handleChange('location')}
                onFocus={this.findPosition}
                placeholder={I18n.t('screens.profileSettings.location_hint')}
              />
            </InputGroup>
            <InputGroup title= {I18n.t('screens.profileSettings.contact')} >
              <TextInput
                title={I18n.t('screens.profileSettings.email')}
                value={email}
                onChangeText={this.handleContactChange('email')}
                onFocus={this.findPosition}
                placeholder={I18n.t('screens.profileSettings.contact_hint')}
                keyboardType="email-address"
              />
              <TextInput
                title={I18n.t('screens.profileSettings.phone_number')}
                value={phoneNumber}
                onChangeText={this.handleContactChange('phoneNumber')}
                onFocus={this.findPosition}
                placeholder={I18n.t('screens.profileSettings.phone_hint')}
                keyboardType="phone-pad"
              />
              <TextInput
                noBorder
                title={I18n.t('screens.profileSettings.website')}
                value={website}
                onChangeText={this.handleContactChange('website')}
                onFocus={this.findPosition}
                placeholder={I18n.t('screens.profileSettings.website_hint')}
              />
            </InputGroup>
            <InputGroup title={I18n.t('screens.profileSettings.Aaout')}>
              <TextArea
                noBorder
                value={about}
                onChangeText={this.handleChange('about')}
                onFocus={this.findPosition}
                placeholder={I18n.t('screens.profileSettings.about_hint')}
              />
            </InputGroup>
          </View>
          {keyboardVisible && <EditProfileBanner onSave={this.handleSave} />}
        </KeyboardAwareScrollView>
        {!keyboardVisible && <EditProfileBanner onSave={this.handleSave} />}
      </View>
    );
  }
}

const mapStateToProps = state => ({ profile: _.get(state, 'profile.data') });

const mapDispatchToProps = {
  fetchProfile,
  updateProfile,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ProfileSettings);
