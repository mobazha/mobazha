import React, { Component } from 'react';
import { connect } from 'react-redux';
import { View, Text, ActivityIndicator, TouchableWithoutFeedback } from 'react-native';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import decode from 'unescape';

import {
  primaryTextColor,
  secondaryTextColor,
  starRatingColor,
  foregroundColor,
} from '../commonColors';
import LocationPin from '../atoms/LocationPin';
import ProductSection from '../atoms/ProductSection';
import { fetchProfile } from '../../reducers/profile';
import AvatarImage from '../atoms/AvatarImage';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    marginBottom: 2,
    borderBottomWidth: 0,
  },
  avatarWrapper: {
    flexDirection: 'row',
    paddingLeft: 2,
  },
  avatar: {
    width: 40,
    height: 40,
  },
  contentWrapper: {
    paddingHorizontal: 10,
    flex: 1,
  },
  positionWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-start',
    marginTop: 2,
    marginBottom: 5,
  },
  rating: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
    marginRight: 10,
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
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    textAlign: 'left',
    color: 'rgb(0, 0, 0)',
    paddingBottom: 10,
  },
  showAllButton: {
    flex: 1,
    paddingHorizontal: 17,
    paddingVertical: 8,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
    marginRight: 5,
  },
  showAllText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  buttonsWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  loading: {
    paddingBottom: 10,
  },
};

class SellerInfo extends Component {
  static getDerivedStateFromProps(props) {
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

  render() {
    const { toStore, toMessage } = this.props;
    const { profile } = this.state;
    if (!_.isEmpty(profile)) {
      const avgRating = _.get(profile, 'stats.averageRating') || 0;
      const ratingCount = _.get(profile, 'stats.ratingCount') || 0;
      return (
        <ProductSection style={styles.wrapper} activeOpacity={1} title={I18n.t('components.molecules.SellerInfo.about')}>
          <TouchableWithoutFeedback onPress={toStore}>
            <View style={styles.avatarWrapper}>
              <AvatarImage style={styles.avatar} thumbnail={_.get(profile, 'avatarHashes.small')} />
              <View style={styles.contentWrapper}>
                <Text style={styles.shopName}>{decode(profile.name) || I18n.t('components.molecules.SellerInfo.unknown')}</Text>
                <View style={styles.positionWrapper}>
                  <Text style={styles.rating}>
                    <Ionicons name="md-star" size={16} color={starRatingColor} />
                    &nbsp;
                    {`${avgRating.toFixed(1)} (${ratingCount})`}
                  </Text>
                  <LocationPin secondary style={{ flex: 1 }} location={decode(profile.location)} />
                </View>
                {!_.isEmpty(profile.shortDescription) && (
                  <Text style={styles.description}>
                    {decode(profile.shortDescription)}
                  </Text>)}
              </View>
            </View>
          </TouchableWithoutFeedback>
          <View style={styles.buttonsWrapper}>
            <TouchableWithoutFeedback onPress={toMessage}>
              <View style={styles.showAllButton}>
                <Text style={styles.showAllText}>{I18n.t('components.molecules.SellerInfo.message')}</Text>
              </View>
            </TouchableWithoutFeedback>
            <TouchableWithoutFeedback onPress={toStore}>
              <View style={styles.showAllButton}>
                <Text style={styles.showAllText}>{I18n.t('components.molecules.SellerInfo.visit_store')}</Text>
              </View>
            </TouchableWithoutFeedback>
          </View>
        </ProductSection>
      );
    } else {
      return (
        <View style={[styles.wrapper]}>
          <ActivityIndicator size="large" style={styles.loading} color={secondaryTextColor} />
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
)(SellerInfo);
