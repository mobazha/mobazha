import React, { Component } from 'react';
import { connect } from 'react-redux';
import { View, Text, ActivityIndicator, TouchableWithoutFeedback } from 'react-native';
import FastImage from 'react-native-fast-image';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import decode from 'unescape';

import {
  foregroundColor,
  primaryTextColor,
  secondaryTextColor,
  starRatingColor,
  borderColor,
} from '../commonColors';
import LocationPin from '../atoms/LocationPin';
import { fetchProfile } from '../../reducers/profile';
import { mbzGatewayAPI } from '../../api/const';

import {I18n} from '../../langs/I18n';

const defaultHeader = require('../../assets/images/defaultHeader.png');
const defaultAvatar = require('../../assets/images/defaultAvatar.png');

const styles = {
  wrapper: {
    width: '100%',
    height: 237,
    backgroundColor: foregroundColor,
    borderColor,
    shadowColor: 'rgba(213, 213, 213, 0.8)',
    shadowOffset: {
      width: 0,
      height: 2,
    },
    shadowRadius: 2,
    shadowOpacity: 1,
  },
  image: {
    width: '100%',
    height: '50%',
  },
  compactImage: {
    flex: 1,
  },
  contentWrapper: {
    height: 118,
    paddingBottom: 15,
    paddingHorizontal: 15,
  },
  avatarWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-start',
    paddingHorizontal: 10,
    paddingVertical: 3,
    height: 26,
    marginBottom: 6,
  },
  avatar: {
    width: 24,
    height: 24,
    borderRadius: 12,
    marginRight: 5,
    overflow: 'hidden',
    shadowColor: 'rgba(0, 0, 0, 0.5)',
    shadowOffset: {
      width: 0,
      height: 0,
    },
    shadowRadius: 2,
    shadowOpacity: 1,
  },
  positionWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    height: 23,
    paddingHorizontal: 7,
    marginBottom: 5,
    paddingTop: 5,
  },
  rating: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  location: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  shopName: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  handle: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  description: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    textAlign: 'left',
    color: 'rgb(0, 0, 0)',
    paddingHorizontal: 10,
    height: 34,
  },
};

class ShopCard extends Component {
  static getDerivedStateFromProps(props, state) {
    const { peerID, profiles } = props;
    return { profile: profiles && profiles[peerID] };
  }

  state = {
    profile: null,
  }

  componentDidMount() {
    if (!this.props.profile) {
      const { fetchProfile, peerID } = this.props;
      fetchProfile({ peerID, getLoaded: true });
    }
  }

  shouldComponentUpdate(nextProps, nextState) {
    const { profile } = this.state;
    const nextProfile = nextState.profile;
    if (!profile && nextProfile) {
      return true;
    } else {
      return false;
    }
  }

  handlePress = () => {
    const { onPress, peerID } = this.props;
    onPress(peerID);
  }

  render() {
    let { profile } = this.props; // eslint-disable-line
    const {
      peerID, style, contentWrapperStyle, compact,
    } = this.props;
    if (!profile && !_.isEmpty(peerID)) {
      ({profile} = this.state); // eslint-disable-line
    }
    if (!_.isEmpty(profile)) {
      const cover = _.hasIn(profile, 'headerHashes')
        ? { uri: `${mbzGatewayAPI}/ob/images/${profile.headerHashes.large}` }
        : defaultHeader;
      const avatar = _.hasIn(profile, 'avatarHashes')
        ? { uri: `${mbzGatewayAPI}/ob/images/${profile.avatarHashes.small}` }
        : defaultAvatar;
      const avgRating = _.get(profile, 'stats.averageRating') || 0;
      const ratingCount = _.get(profile, 'stats.ratingCount') || 0;
      return (
        <TouchableWithoutFeedback activeOpacity={1} onPress={this.handlePress}>
          <View style={[styles.wrapper, style]}>
            <FastImage
              style={compact ? styles.compactImage : styles.image}
              source={cover}
              resizeMode="cover"
            />
            {!compact && (
              <View style={[styles.contentWrapper, contentWrapperStyle]}>
                <View style={styles.avatarWrapper}>
                  <FastImage style={styles.avatar} source={avatar} resizeMode="cover" />
                  <Text style={styles.shopName}>{decode(profile.name) || I18n.t('components.molecules.ShopCard.unknown')}</Text>
                  {/* <Text style={styles.handle}>
                    &nbsp;&nbsp;@
                    {profile.handle}
                  </Text> */}
                </View>
                <Text style={styles.description} numberOfLines={2}>
                  {decode(profile.shortDescription)}
                </Text>
                <View style={styles.positionWrapper}>
                  <LocationPin style={{ flex: 1 }} location={decode(profile.location)} />
                  <Text style={styles.rating}>
                    &nbsp;
                    <Ionicons name="md-star" size={16} color={starRatingColor} />
                    &nbsp;
                    {`${avgRating.toFixed(1)} (${ratingCount})`}
                  </Text>
                </View>
              </View>
            )}
          </View>
        </TouchableWithoutFeedback>
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
});

const mapDispatchToProps = {
  fetchProfile,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ShopCard);
