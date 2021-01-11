import React, { Component } from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback, Dimensions } from 'react-native';
import { isEmpty } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import decode from 'unescape';
import FastImage from 'react-native-fast-image';

import { fetchProfile } from '../../reducers/profile';

import {
  foregroundColor,
  secondaryTextColor,
  starRatingColor,
  staticLabelColor,
  borderColor,
} from '../commonColors';
import { priceStyle } from '../commonStyles';

import { getImageSourceWithDefault } from '../../utils/files';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const { width: screenWidth } = Dimensions.get('screen');

const styles = {
  wrapper: {
    height: (screenWidth / 3) + 1,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderBottomWidth: 1,
    borderColor,
    flexDirection: 'row',
  },
  image: {
    width: screenWidth / 3,
    height: screenWidth / 3,
  },
  title: {
    fontSize: 15,
    fontWeight: '600',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#000000',
  },
  textWrapper: {
    paddingHorizontal: 12,
    flex: 1,
    justifyContent: 'space-evenly',
  },
  rating: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  priceWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  shippingTextWrapper: {
    borderRadius: 1,
    backgroundColor: '#00BF65',
    height: 16,
    paddingHorizontal: 2,
    alignItems: 'center',
    justifyContent: 'center',
  },
  shipping: {
    fontSize: 11,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#ffffff',
  },
  fromText: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: staticLabelColor,
  },
};

class ProductListItem extends Component {
  static getDerivedStateFromProps(props) {
    const { peerID, profiles, fetchedProfile } = props;
    return fetchedProfile ? {} : { profile: profiles && profiles[peerID] };
  }

  state = {
    profile: null,
  }

  componentDidMount() {
    const { peerID, fetchProfile, fetchedProfile } = this.props;
    if (!fetchedProfile) {
      fetchProfile({ peerID, getLoaded: true });
    }
  }

  shouldComponentUpdate(nextProps, nextState) {
    const { rates, localCurrency } = this.props;
    const { profile } = this.state;
    const nextProfile = nextState.profile;
    if (!profile && nextProfile) {
      return true;
    } else if (rates[localCurrency] !== nextProps.rates[nextProps.localCurrency]) {
      return true;
    } else {
      return false;
    }
  }

  onPressListing = () => {
    const { peerID, slug, hash, onPress } = this.props;
    if (!onPress) {
      return;
    }

    if (peerID) {
      if (hash) {
        onPress({ hash, peerID, slug });
      } else {
        onPress({ peerID, slug });
      }
    } else {
      onPress({ slug });
    }
  }

  getStoreName() {
    const {
      fetchedProfile, profile, peerID,
    } = this.props;
    if (!fetchedProfile) {
      if (isEmpty(peerID)) {
        return profile.name;
      }

      if (this.state.profile) {
        return this.state.profile.name;
      }
      return '';
    }
    return fetchedProfile.name;
  }
  render() {
    const {
      freeShipping,
      thumbnail,
      amount,
      currencyCode,
      averageRating,
      ratingCount,
      title,
      style,
      localLabelFromBCH,
      hideSellerName,
    } = this.props;
    const freeShippingText = isEmpty(freeShipping) ? '' : 'FREE SHIPPING';
    return (
      <TouchableWithoutFeedback onPress={this.onPressListing}>
        <View style={[styles.wrapper, style]}>
          <FastImage
            style={styles.image}
            source={getImageSourceWithDefault(thumbnail)}
            resizeMode={FastImage.resizeMode.cover}
          />
          <View style={styles.textWrapper}>
            <View>
              <Text style={styles.title} numberOfLines={2}>
                {decode(title)}
              </Text>
              {!hideSellerName && (
                <Text style={styles.fromText} numberOfLines={1}>
                  {decode(this.getStoreName())}
                </Text>
              )}
            </View>
            <View style={styles.priceWrapper}>
              <Text style={priceStyle}>{`${localLabelFromBCH(amount, currencyCode)}  `}</Text>
              {!isEmpty(freeShipping) && (
                <View style={styles.shippingTextWrapper}>
                  <Text style={[priceStyle, styles.shipping]}>{` ${freeShippingText} `}</Text>
                </View>
              )}
            </View>
            <Text style={styles.rating}>
              <Ionicons name="md-star" size={16} color={starRatingColor} />
              &nbsp;
              {`${averageRating.toFixed(1)} (${ratingCount})`}
            </Text>
          </View>
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  profile: state.profile.data || {},
  ...convertorsMap(state),
  rates: state.exchangeRate.rates,
});

const mapDispatchToProps = {
  fetchProfile,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ProductListItem);
