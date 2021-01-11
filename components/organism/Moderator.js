import React, { Component } from 'react';
import { connect } from 'react-redux';
import { View, Text } from 'react-native';
import * as _ from 'lodash';
import decode from 'unescape';
import deepEqual from 'deep-equal';
import Octicons from 'react-native-vector-icons/Octicons';
import * as striptags from 'striptags';

import { fetchProfile } from '../../reducers/profile';
import { convertorsMap } from '../../selectors/currency';
import AvatarImage from '../atoms/AvatarImage';
import { brandColor, primaryTextColor, formLabelColor } from '../commonColors';
import LocationPin from '../atoms/LocationPin';
import Fee from '../atoms/Fee';
import { eatSpaces } from '../../utils/string';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
  },
  mainContent: {
    flex: 1,
    marginLeft: 8,
    flexDirection: 'column',
  },
  img: {
    width: 36,
    height: 36,
  },
  rowContainer: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  name: {
    flex: 1,
    fontSize: 16,
    fontWeight: 'bold',
    marginRight: 7,
    color: primaryTextColor,
  },
  verified: {
    marginLeft: 3,
    fontSize: 13,
    fontWeight: 'bold',
    color: brandColor,
  },
  locationText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: 'rgb(0, 0, 0)',
    marginRight: 12,
  },
  briefInfo: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  description: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    textAlign: 'left',
    paddingVertical: 10,
    color: 'rgb(0, 0, 0)',
    marginRight: 16,
  },
  location: {
    paddingTop: 0,
    paddingBottom: 0,
    paddingRight: 10,
    color: formLabelColor,
    fontSize: 14,
    flexShrink: 1,
  },
  fee: {
    color: formLabelColor,
  },
};

class Moderator extends Component {
  static getDerivedStateFromProps(props, state) {
    const { peerID, profiles } = props;
    if (peerID) {
      return { profile: profiles && profiles[peerID] };
    } else {
      return {};
    }
  }

  state = {
    profile: null,
  };

  componentDidMount() {
    const { fetchProfile, peerID } = this.props;
    fetchProfile({ peerID, getLoaded: true });
  }

  shouldComponentUpdate(nextProps, nextState) {
    const { profile } = this.state;
    const nextProfile = nextState.profile;
    if (deepEqual(profile, nextProfile)) {
      return false;
    } else {
      return true;
    }
  }

  checkIfVerified = () => {
    const { profile } = this.state;
    const { moderators } = this.props;
    const peerID = _.get(profile, 'peerID');
    return moderators.find(moderator => moderator === peerID);
  }

  render() {
    const { profile } = this.state;
    const moderatorInfo = _.get(profile, 'moderatorInfo');
    const { fee, description } = moderatorInfo || {};
    const name = _.get(profile, 'name', '');
    const location = _.get(profile, 'location');
    const isVerified = this.checkIfVerified();
    if (!profile) {
      return (
        <View style={styles.wrapper}>
          <AvatarImage style={styles.img} />
          <View style={styles.mainContent}>
            <Text style={styles.name}>
            {I18n.t('components.organism.Moderator.unknown_moderator')}
            </Text>
            <Text style={styles.description}>
            {I18n.t('components.organism.Moderator.unknown_moderator_description')}              
            </Text>
          </View>
        </View>
      );
    }

    return (
      <View style={styles.wrapper}>
        <AvatarImage
          style={styles.img}
          thumbnail={_.get(profile, 'avatarHashes.tiny')}
        />
        <View style={styles.mainContent}>
          <View style={styles.rowContainer}>
            <Text style={styles.name}>
              {decode(name)}
            </Text>
            {isVerified && (
              <React.Fragment>
                <Octicons name="verified" size={14} color={brandColor} />
                <Text style={styles.verified}>{I18n.t('components.organism.Moderator.verified')}  </Text>
              </React.Fragment>
            )}
          </View>

          <Text style={styles.description} numberOfLines={3} ellipsizeMode="tail">
            {description && eatSpaces(decode(striptags.default(description)))}
          </Text>
          <View style={styles.briefInfo}>
            {!_.isEmpty(location) && (
              <LocationPin style={styles.location} location={decode(location)} />
            )}
            {fee && (
              <Fee moderatorInfo={fee} style={styles.fee} />
            )}
          </View>
        </View>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  moderators: state.moderators,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  fetchProfile,
};

export default connect(mapStateToProps, mapDispatchToProps)(Moderator);
