import React from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import { withNavigation } from 'react-navigation';
import { get, isEmpty } from 'lodash';
import decode from 'unescape';

import AvatarImage from '../atoms/AvatarImage';
import FollowButton from '../atoms/FollowButton';
import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 11,
  },
  content: {
    flex: 1,
    flexDirection: 'column',
    alignItems: 'flex-start',
    justifyContent: 'center',
    paddingRight: 20,
  },
  name: {
    fontSize: 14,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  text: {
    fontSize: 14,
    color: primaryTextColor,
  },
  avatarImage: {
    width: 38,
    height: 38,
    margin: 4,
    marginRight: 13,
  },
};

class FeedListItem extends React.Component {
  getProfile = () => {
    const { item } = this.props;
    return get(item, 'user.data', {});
  }

  navigateToExternalStore = () => {
    const { profile: currentProfile, navigation } = this.props;
    const { peerID } = this.getProfile();
    if (peerID !== get(currentProfile, 'data.peerID')) {
      navigation.push('ExternalStore', { peerID });
    }
  }

  render() {
    const { profile: currentProfile } = this.props;
    const profile = this.getProfile();
    const { peerID } = profile;
    const { name = 'Anonymous', shortDescription = '' } = profile || {};
    return (
      <TouchableWithoutFeedback onPress={this.navigateToExternalStore}>
        <View style={styles.wrapper}>
          <AvatarImage
            style={styles.avatarImage}
            thumbnail={get(profile, 'avatarHashes.tiny')}
            onPress={this.onPressAvatar}
          />
          <View style={styles.content}>
            <Text style={styles.name}>{decode(name)}</Text>
            {!isEmpty(decode(shortDescription)) && (
              <Text style={styles.text} numberOfLines={2} ellipsizeMode="tail">{decode(shortDescription)}</Text>
            )}
          </View>
          {peerID !== get(currentProfile, 'data.peerID') && (<FollowButton type="smallBrandButton" peerID={peerID} />)}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = ({ profile }) => ({ profile });

export default withNavigation(connect(mapStateToProps)(FeedListItem));
