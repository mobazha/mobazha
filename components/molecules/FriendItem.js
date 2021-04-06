import React, { Component } from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import { withNavigation } from 'react-navigation';
import * as _ from 'lodash';
import decode from 'unescape';

import { getProfile } from '../../api/profile';
import { cellStyles, chatStyles } from '../../utils/styles';
import { foregroundColor } from '../commonColors';
import AvatarImage from '../atoms/AvatarImage';
import FollowButton from '../atoms/FollowButton';
import HollowButton from '../atoms/HollowButton';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    height: 78,
    backgroundColor: foregroundColor,
    flexDirection: 'column',
  },
  contentContainer: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#fff',
  },
  descriptionContainer: {
    flex: 1,
    flexDirection: 'column',
    alignItems: 'flex-start',
    backgroundColor: '#fff',
    paddingVertical: 6,
    paddingLeft: 5,
    paddingRight: 10,
  },
  avatarImage: {
    width: 45,
    height: 45,
    marginVertical: 6,
    marginLeft: 13,
    marginRight: 5,
  },
  followButton: {
    width: 30,
    height: 30,
    marginRight: 13,
  },
  bigPadding: {
    paddingVertical: 6,
  },
  name: {
    fontSize: 15,
    fontWeight: '600',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#000000',
  },
  shortDescription: {
    fontSize: 15,
    fontWeight: '400',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#000000',
  },
};

class FriendItem extends Component {
  state = {}

  onPressAvatar = () => {
    const { item, navigation } = this.props;
    const { peerID } = item;
    navigation.push('ExternalStore', { peerID });
  }

  UNSAFE_componentWillMount() {
    const { peerID } = this.props;
    getProfile(peerID).then((response) => {
      this.setState({
        profile: response,
      });
    });
  }

  render() {
    const {
      peerID,
      // item: profile,
      style,
      onPress,
      isFirst,
      isLast,
      showFollowButton,
      showMessageButton,
      fromFollowing,
    } = this.props;
    const { profile } = this.state;

    const name = _.get(profile, 'name', '');
    const handle = _.get(profile, 'handle', '');
    const shortDescription = _.get(profile, 'shortDescription', '');
    return (
      <TouchableWithoutFeedback onPress={onPress}>
        <View style={[styles.wrapper, style]}>
          {isFirst && <View style={cellStyles.separator} />}
          <View style={[styles.contentContainer, showMessageButton && styles.bigPadding]}>
            <AvatarImage
              style={styles.avatarImage}
              thumbnail={_.get(profile, 'avatarHashes.tiny', '') || _.get(profile, 'avatar.tiny', '')}
              onPress={this.onPressAvatar}
            />
            <View style={styles.descriptionContainer}>
              <Text style={styles.name} numberOfLines={1}>
                {decode(name)}
              </Text>
              {!!handle && (
                <Text style={chatStyles.handle} numberOfLines={1}>
                  {` @${handle}`}
                </Text>
              )}
              {!!shortDescription && (
              <Text style={styles.shortDescription} numberOfLines={2}>
                {`${decode(shortDescription)}`}
              </Text>
            )}
            </View>
            {showFollowButton && <FollowButton type="smallWhiteButton" style={styles.followButton} peerID={peerID} fromFollowing={fromFollowing} />}
            {showMessageButton && (
              <HollowButton
                style={chatStyles.messageButton}
                small
                title="Message"
                onPress={onPress}
              />
            )}
          </View>
          {isLast && <View style={cellStyles.separator} />}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

export default withNavigation(FriendItem);
