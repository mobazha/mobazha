import React, { Component } from 'react';
import { connect } from 'react-redux';
import { View, Text, ActivityIndicator, TouchableWithoutFeedback, Dimensions } from 'react-native';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { withNavigation } from 'react-navigation';
import decode from 'unescape';
import ImageViewer from 'react-native-image-zoom-viewer';

import ModalBackButton from '../atoms/ModalBackButton';
import AvatarImage from '../atoms/AvatarImage';
import FollowButton from '../atoms/FollowButton';
import HollowButton from '../atoms/HollowButton';
import { OBDarkModal } from '../templates/OBModal';

import {
  foregroundColor,
  primaryTextColor,
  secondaryTextColor,
  starRatingColor,
  brandColor,
} from '../commonColors';
import LocationPin from '../atoms/LocationPin';
import { fetchProfile } from '../../reducers/profile';
import { getImageSourceForImageViewer } from '../../utils/files';

import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const styles = {
  wrapper: {
    width: '100%',
    backgroundColor: 'transparent',
  },
  cover: {
    width: '100%',
    height: SCREEN_WIDTH / 3,
    backgroundColor: 'transparent',
  },
  contentWrapper: {
    paddingHorizontal: 15,
    paddingBottom: 6,
    backgroundColor: foregroundColor,
  },
  avatarWrapper: {
    width: '100%',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
  },
  avatarWrapperBackground: {
    position: 'absolute',
    backgroundColor: 'white',
    left: 0,
    right: 0,
    top: 32,
    bottom: 0,
  },
  ratingsWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    backgroundColor: foregroundColor,
  },
  avatar: {
    width: 64,
    height: 64,
  },
  chips: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-around',
  },
  location: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
    marginVertical: 12,
  },
  shopName: {
    fontSize: 15,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  locationPin: {
    flex: 1,
    marginTop: 8,
  },
  chipValue: {
    fontSize: 18,
    fontWeight: 'bold',
    color: primaryTextColor,
    alignItems: 'center',
  },
  readmore: {
    marginVertical: 5,
  },
  moreless: {
    paddingTop: 5,
    color: 'rgb(0, 122, 255)',
  },
  description: {
    fontSize: 15,
    minHeight: 20,
    color: primaryTextColor,
  },
  chip: {
    alignItems: 'center',
    fontSize: 15,
  },
  chipLabel: {
    fontSize: 14,
    fontWeight: 'normal',
    color: 'rgb(151, 151, 151)',
    marginTop: 2,
  },
  button: {
    height: 30,
    justifyContent: 'center',
    alignItems: 'center',
    borderWidth: 1,
    borderRadius: 2,
  },
  editButton: {
    width: '100%',
    color: 'black',
    borderRadius: 2,
    borderColor: '#c8c7cc',
  },
  buttonsContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: 12,
    marginBottom: 8,
  },
  chatButton: {
    borderColor: brandColor,
    width: '48%',
  },
  chatButtonText: {
    color: brandColor,
    fontWeight: 'bold',
  },
  followButton: {
    minWidth: '48%',
  },
  iconButton: {
    color: brandColor,
  },
  title: {
    flex: 1,
    fontSize: 18,
    lineHeight: 18,
    fontWeight: 'bold',
    textAlign: 'center',
    color: primaryTextColor,
    paddingTop: 16,
    width: '100%',
  },
};

const SocialChip = ({ label, children, onPress }) => (
  <TouchableWithoutFeedback onPress={onPress}>
    <View style={styles.chip}>
      {children}
      <Text style={styles.chipLabel}>{label}</Text>
    </View>
  </TouchableWithoutFeedback>
);

class ShopInfo extends Component {
  static getDerivedStateFromProps(props) {
    const { peerID, profiles, userProfile } = props;
    if (peerID === userProfile.peerID || !peerID) {
      return { profile: userProfile, isMyStore: true };
    }
    return { profile: profiles && profiles[peerID], isMyStore: false };
  }

  state = {
    imagesForModal: [],
  };

  componentDidMount() {
    const { fetchProfile, peerID, profile } = this.props;
    if (!profile) {
      fetchProfile({ peerID });
    }
  }

  shouldComponentUpdate(nextProps, nextState) {
    const { profile, imagesForModal } = this.state;
    if (
      (!profile && nextState.profile) ||
      !_.isEqual(this.props.profile && nextProps.profile) ||
      (imagesForModal.length !== nextState.imagesForModal.length)
    ) {
      return true;
    } else {
      return false;
    }
  }

  getProfile = () => {
    const { profile, peerID } = this.props;
    if (!profile && !_.isEmpty(peerID)) {
      return this.state.profile;
    }
    return profile;
  }

  getProfileName = () => {
    const profile = this.getProfile();
    return decode(profile.name) || I18n.t('components.molecules.ShopInfo.unknown');
  }

  handleClose = () => {
    this.setState({ imagesForModal: [] });
  };

  handleEdit = () => {
    this.props.navigation.navigate('ProfileSettings');
  };

  handlePressReviews = () => {
    const profile = this.getProfile();
    const { peerID } = this.props;

    this.props.navigation.navigate({
      routeName: 'StoreRatings',
      params: { peerID, profile },
    });
  };

  handlePressFollowings = () => {
    const { peerID, externalStore } = this.props;
    eventTracker.trackEvent('UserProfile-TappedFollowings');
    this.props.navigation.push(
      externalStore ? 'ExternalFollowings' : 'Followings',
      { peerID, name: this.getProfileName() },
    );
  };

  handlePressFollowers = () => {
    const { peerID, externalStore } = this.props;
    eventTracker.trackEvent('UserProfile-TappedFollowers');
    this.props.navigation.push(
      externalStore ? 'ExternalFollowers' : 'Followers',
      { peerID, name: this.getProfileName() },
    );
  };

  handlePressImage = (image) => {
    if (image) {
      this.setState({ imagesForModal: [getImageSourceForImageViewer(image)] });
    }
  };

  navigateToChatDetail = () => {
    const { peerID, navigation } = this.props;
    eventTracker.trackEvent('Chat-StartConveration');
    eventTracker.trackEvent('UserProfile-TappedMessage');
    navigation.navigate('ChatDetail', { peerID, subject: '' });
  }

  render() {
    const {
      peerID, style, externalStore,
    } = this.props;
    const profile = this.getProfile();
    const { imagesForModal, isMyStore } = this.state;

    if (!_.isEmpty(profile)) {
      const avatar = _.get(profile, 'avatarHashes.large');
      const avgRating = _.get(profile, 'stats.averageRating') || 0;
      const ratingCount = _.get(profile, 'stats.ratingCount') || 0;
      const followerCount = _.get(profile, 'stats.followerCount') || 0;
      const followingCount = _.get(profile, 'stats.followingCount') || 0;
      const shortDescription = _.get(profile, 'shortDescription');

      return (
        <View style={[styles.wrapper, style]}>
          <View style={styles.avatarWrapper}>
            <View style={styles.avatarWrapperBackground} />
            <AvatarImage
              style={styles.avatar}
              thumbnail={avatar}
              onPress={() => this.handlePressImage(avatar)}
              showLocal={!externalStore}
            />
            <Text style={styles.title} numberOfLines={1}>
              {decode(_.get(profile, 'name'))}
            </Text>
          </View>
          <View style={styles.ratingsWrapper}>
            <View style={styles.chips}>
              <SocialChip label={I18n.t('components.molecules.ShopInfo.reviews', {count: ratingCount})} onPress={this.handlePressReviews}>
                <Text style={styles.chipValue}>
                  <Ionicons name="md-star" size={20} color={starRatingColor} />
                  &nbsp;
                  {avgRating.toFixed(1)}
                </Text>
              </SocialChip>
              <SocialChip label={I18n.t('components.molecules.ShopInfo.following')} onPress={this.handlePressFollowings}>
                <Text style={styles.chipValue}>{followingCount}</Text>
              </SocialChip>
              <SocialChip label={I18n.t('components.molecules.ShopInfo.followers')} onPress={this.handlePressFollowers}>
                <Text style={styles.chipValue}>{followerCount}</Text>
              </SocialChip>
            </View>
          </View>
          <View style={[styles.contentWrapper]}>
            {!_.isEmpty(shortDescription) && (
            <Text style={styles.description}>
              {decode(shortDescription)}
            </Text>)}
            <LocationPin style={styles.locationPin} location={decode(profile.location)} />
            {isMyStore ? (
              <View style={styles.buttonsContainer}>
                <HollowButton title={I18n.t('components.molecules.ShopInfo.edit_profile')} onPress={this.handleEdit} style={[styles.button, styles.editButton]} />
              </View>
            ) : (
              <View style={styles.buttonsContainer}>
                <TouchableWithoutFeedback
                  onPress={this.navigateToChatDetail}
                >
                  <View style={[styles.button, styles.chatButton]}>
                    <Text style={styles.chatButtonText}>{I18n.t('components.molecules.ShopInfo.message')}</Text>
                  </View>
                </TouchableWithoutFeedback>
                <FollowButton type="fullButton" style={styles.followButton} peerID={peerID} />
              </View>
              )}
          </View>
          <OBDarkModal
            onRequestClose={this.handleClose}
            visible={imagesForModal.length > 0}
            darkContent
          >
            <ImageViewer
              imageUrls={imagesForModal}
              enableSwipeDown
              enablePreload
              onCancel={this.handleClose}
            />
            <ModalBackButton onPress={this.handleClose} />
          </OBDarkModal>
        </View>
      );
    } else {
      return (
        <View style={[styles.wrapper, style]}>
          <ActivityIndicator size="large" color={secondaryTextColor} />
        </View>
      );
    }
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  userProfile: state.profile.data,
});

const mapDispatchToProps = {
  fetchProfile,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(withNavigation(ShopInfo));
