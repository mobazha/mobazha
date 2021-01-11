import React, { Component } from 'react';
import { connect } from 'react-redux';
import { Text, TouchableWithoutFeedback, View, ActivityIndicator } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import Toast from 'react-native-root-toast';

import { followPeer, unfollowPeer } from '../../reducers/follow';
import { greenColor, brandColor } from '../commonColors';
import { eventTracker } from '../../utils/EventTracker';
import { TOAST_OPTION } from '../../utils/toast';

import {I18n} from '../../langs/I18n';

const fullButtonStyles = {
  button: {
    height: 30,
    justifyContent: 'center',
    alignItems: 'center',
    borderWidth: 1,
    borderRadius: 2,
    paddingHorizontal: 10,
  },
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  checkmark: {
    color: greenColor,
    marginRight: 3,
    width: 14,
  },
  unfollowingButtonText: {
    color: brandColor,
    fontWeight: 'bold',
  },
  followingButtonText: {
    color: 'white',
    fontWeight: 'bold',
  },
  activityIndicator: {
    marginLeft: 10,
  },
  iconButton: {
    color: 'white',
  },
};

const smallButtonStyles = {
  button: {
    height: 30,
    width: 30,
    justifyContent: 'center',
    alignItems: 'center',
    borderWidth: 1,
    borderRadius: 15,
  },
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  checkmark: {
    color: 'white',
    width: 14,
  },
  unfollowingButtonText: {
    color: 'black',
    fontWeight: 'bold',
  },
  followingButtonText: {
    color: 'white',
    fontWeight: 'bold',
  },
  iconButton: {
    color: brandColor,
  },
};

class FollowButton extends Component {
  static getDerivedStateFromProps(props) {
    const {
      peerID, followingsFromLocal, followQueue, unfollowQueue,
    } = props;
    const isFollowing = followingsFromLocal.includes(peerID);
    const isInFollowQueue = followQueue.includes(peerID);
    const isInUnfollowQueue = unfollowQueue.includes(peerID);
    return { isFollowing, isInFollowQueue, isInUnfollowQueue };
  }

  state = {
    isFollowing: false,
  }

  shouldComponentUpdate(nextProps, nextState) {
    const { isFollowing, isInFollowQueue, isInUnfollowQueue } = this.state;
    const {
      isFollowing: nextIsFollowing,
      isInFollowQueue: nextIsInFollowQueue,
      isInUnfollowQueue: nextIsInUnfollowQueue,
    } = nextState;
    if (
      isFollowing !== nextIsFollowing
      || isInFollowQueue !== nextIsInFollowQueue
      || isInUnfollowQueue !== nextIsInUnfollowQueue
    ) {
      return true;
    } else {
      return false;
    }
  }

  componentDidUpdate(prevProps, prevState) {
    const { type } = this.props;
    if (type === 'smallWhiteButton') {
      const { isFollowing: prevIsFollowing } = prevState;
      const { isFollowing } = this.state;

      if (isFollowing && !prevIsFollowing) {
        Toast.show('Followed!', TOAST_OPTION);
      } else if (!isFollowing && prevIsFollowing) {
        Toast.show('Unfollowed', TOAST_OPTION);
      }
    }
  }

  handlePress = () => {
    const {
      peerID, followPeer, unfollowPeer,
    } = this.props;
    const { isFollowing, isInFollowQueue, isInUnfollowQueue } = this.state;
    if (isFollowing && !isInUnfollowQueue) {
      eventTracker.trackEvent('UserProfile-UnfollowedUser');
      unfollowPeer(peerID);
    } else if (!isInFollowQueue) {
      eventTracker.trackEvent('UserProfile-FollowedUser');
      followPeer(peerID);
    }
  };

  renderFullButton() {
    const { style } = this.props;
    const { isFollowing, isInFollowQueue, isInUnfollowQueue } = this.state;

    return (
      <TouchableWithoutFeedback
        onPress={this.handlePress}
      >
        <View
          style={[
            fullButtonStyles.button,
            {
              backgroundColor: isFollowing ? 'white' : brandColor,
              borderColor: brandColor,
            },
            style,
          ]}
        >
          {isFollowing ? (
            <View style={fullButtonStyles.wrapper}>
              <Ionicons name="ios-checkmark" size={28} style={fullButtonStyles.checkmark} />
              <Text style={fullButtonStyles.unfollowingButtonText}>{I18n.t('components.atoms.FollowButton.following')}</Text>
              {isInUnfollowQueue && (
                <ActivityIndicator style={fullButtonStyles.activityIndicator} size="small" color={brandColor} />
              )}
            </View>
          ) : (
            <View style={fullButtonStyles.wrapper}>
              <Ionicons size={16} color="#B7B7B7" name="md-person-add" style={fullButtonStyles.iconButton} />
              <Text style={fullButtonStyles.followingButtonText}>  {I18n.t('components.atoms.FollowButton.follow')}</Text>
              {isInFollowQueue && (
                <ActivityIndicator style={fullButtonStyles.activityIndicator} size="small" color="white" />
              )}
            </View>
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }

  renderSmallWhite() {
    const {
      style, profile, peerID,
    } = this.props;
    const { isFollowing, isInFollowQueue, isInUnfollowQueue } = this.state;

    if (peerID === profile.peerID) {
      return null;
    }
    return (
      <TouchableWithoutFeedback
        onPress={this.handlePress}
      >
        <View
          style={[
            smallButtonStyles.button,
            {
              backgroundColor: isFollowing ? 'white' : brandColor,
              borderColor: brandColor,
            },
            style,
          ]}
        >
          {isFollowing ? (
            <View style={smallButtonStyles.wrapper}>
              {!isInUnfollowQueue && (
                <Ionicons name="md-checkmark" size={18} color={brandColor} />
              )}
              {isInUnfollowQueue && (
                <ActivityIndicator size="small" color={brandColor} />
              )}
            </View>
          ) : (
            <View style={smallButtonStyles.wrapper}>
              {!isInFollowQueue && (
                <Ionicons name="md-person-add" size={16} color="white" />
              )}
              {isInFollowQueue && (
                <ActivityIndicator size="small" color="white" />
              )}
            </View>
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }

  renderSmallBrand() {
    const { style } = this.props;
    const { isFollowing, isInFollowQueue, isInUnfollowQueue } = this.state;

    return (
      <TouchableWithoutFeedback
        onPress={this.handlePress}
      >
        <View
          style={[
            smallButtonStyles.button,
            {
              backgroundColor: isFollowing ? brandColor : 'white',
              borderColor: brandColor,
            },
            style,
          ]}
        >
          {isFollowing ? (
            <View style={[smallButtonStyles.wrapper, smallButtonStyles.followingWrapper]}>
              {isInUnfollowQueue ? (
                <ActivityIndicator size="small" color="white" />
              ) : (
                <Ionicons name="ios-checkmark" size={28} style={smallButtonStyles.checkmark} />
              )}
            </View>
          ) : (
            <View style={smallButtonStyles.wrapper}>
              {isInFollowQueue ? (
                <ActivityIndicator size="small" color={brandColor} />
              ) : (
                <Ionicons size={16} color="#B7B7B7" name="md-person-add" style={smallButtonStyles.iconButton} />
              )}
            </View>
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }

  render() {
    const { type } = this.props;
    switch (type) {
      case 'smallBrandButton':
        return this.renderSmallBrand();
      case 'smallWhiteButton':
        return this.renderSmallWhite();
      case 'fullButton':
      default:
        return this.renderFullButton();
    }
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
  followingsFromLocal: state.follow.followingsFromLocal,
  followQueue: state.followQueue.followQueue,
  unfollowQueue: state.followQueue.unfollowQueue,
});

const mapDispatchToProps = {
  followPeer,
  unfollowPeer,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(FollowButton);
