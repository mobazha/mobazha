import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Alert, Image, Dimensions, View, Text, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import {
  foregroundColor,
  starRatingColor,
} from '../commonColors';
import { priceStyle } from '../commonStyles';
import { removeWishListing } from '../../reducers/wishlist';
import { getImageSourceWithDefault } from '../../utils/files';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

export const styles = {
  wrapper: {
    width: SCREEN_WIDTH * 0.47,
    height: SCREEN_WIDTH * 0.65,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    marginHorizontal: 5,
  },
  compactWrapper: {
    width: (SCREEN_WIDTH - 32) * 0.31,
    height: SCREEN_WIDTH * 0.38,
    borderRadius: 2,
    backgroundColor: foregroundColor,
  },
  columnWrapperStyle: {
    marginHorizontal: SCREEN_WIDTH * 0.01,
    paddingBottom: 10,
    justifyContent: 'space-between',
  },
  compactColumnWrapperStyle: {
    justifyContent: 'space-between',
    paddingTop: 12,
  },
  image: {
    width: '100%',
    height: '75%',
  },
  compactImage: {
    width: '100%',
    height: '80%',
  },
  textWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    flex: 1,
    paddingTop: 3,
  },
  ratingWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    flex: 1,
  },
  rating: {
    fontSize: 12,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#8A8A8F',
  },
  price: {
    fontSize: 14,
  },
  compactPrice: {
    fontSize: 12,
  },
  title: {
    fontSize: 13,
    color: '#404040',
  },
  btn: {
    position: 'absolute',
    backgroundColor: 'white',
    width: 32,
    height: 32,
    borderRadius: 2,
    borderWidth: 1,
    borderColor: '#e8e8e8',
    justifyContent: 'center',
    alignItems: 'center',
    elevation: 3,
    right: 6,
    top: 6,
  },
  icon: {
    width: 20,
    height: 20,
    textAlign: 'center',
  },
};

class ProductCard extends PureComponent {
  onPressListing = () => {
    const { peerID, slug, hash, onPress } = this.props;
    if (!onPress) {
      return;
    }
    onPress({ peerID, slug, hash });
  }

  removeFromWishList = () => {
    const { listings, slug, removeWishListing } = this.props;
    const listingInfo = listings.find(item => item.listing.slug === slug);
    removeWishListing(listingInfo);
  }

  handleUnlike = () => {
    Alert.alert('Are you sure?', 'Remove the listing from your wishlist?', [
      { text: 'Cancel' },
      { text: 'OK', onPress: this.removeFromWishList },
    ]);
  }

  render() {
    const {
      thumbnail,
      style,
      averageRating,
      ratingCount,
      title,
      currencyCode,
      amount,
      localLabelFromBCH,
      isWishlist,
      compact,
    } = this.props;

    const wrapperStyle = compact ? styles.compactWrapper : styles.wrapper;

    return (
      <TouchableWithoutFeedback activeOpacity={1} onPress={this.onPressListing}>
        <View style={[wrapperStyle, style]}>
          <Image
            style={compact ? styles.compactImage : styles.image}
            source={getImageSourceWithDefault(thumbnail)}
            resizeMode="cover"
          />
          {!compact && (
            <View style={styles.textWrapper}>
              <Text style={styles.title} numberOfLines={1}>{title}</Text>
            </View>
          )}
          <View style={styles.ratingWrapper}>
            <Text style={[priceStyle, compact ? styles.compactPrice : styles.price]}>
              {amount ? localLabelFromBCH(amount, currencyCode) : ''}
            </Text>
          </View>
          {!compact && (
            <View style={styles.ratingWrapper}>
              {averageRating > 0 && (
                <Text style={styles.rating}>
                  <Ionicons name="md-star" size={16} color={starRatingColor} />
                  &nbsp;
                  {`${averageRating.toFixed(1)} (${ratingCount})`}
                </Text>
              )}
            </View>
          )}
          {isWishlist && (
            <TouchableWithoutFeedback onPress={this.handleUnlike}>
              <View style={styles.btn}>
                <Ionicons style={styles.icon} name="md-heart" size={20} color="rgb(255, 59, 48)" />
              </View>
            </TouchableWithoutFeedback>
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  listings: state.listings.data,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  removeWishListing,
};

export default connect(mapStateToProps, mapDispatchToProps)(ProductCard);
