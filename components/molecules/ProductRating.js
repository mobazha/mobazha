import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, TouchableWithoutFeedback } from 'react-native';
import { isEmpty, get } from 'lodash';

import BuyerReview from './BuyerReview';
import Rating from '../atoms/Rating';

import { foregroundColor } from '../commonColors';

import { getProfile } from '../../api/profile';
import { getRating } from '../../api/products';
import ListingReview from './ListingReview';
import { timeSince } from '../../utils/time';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    paddingVertical: 12,
    backgroundColor: foregroundColor,
  },
  reviewText: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#000000',
  },
  scoreWrapper: {
    marginLeft: 48,
  },
};

class ProductRating extends PureComponent {
  state = {}

  UNSAFE_componentWillMount() {
    const { username, password, nodeID } = this.props;
    getRating(username, password, nodeID).then((response) => {
      if (response.success !== false) {
        this.setState({
          ratingData: response.ratingData,
        });
        const buyerID = get(response, 'ratingData.buyerID.peerID');
        if (buyerID) {
          getProfile(buyerID).then((response) => {
            this.setState({
              profile: response,
            });
          });
        }
      }
    });
  }

  render() {
    if (isEmpty(this.state.ratingData)) {
      return null;
    }
    const { profile, ratingData } = this.state;
    const {
      review,
      overall,
      quality,
      description,
      deliverySpeed,
      customerService,
      timestamp: { seconds },
      vendorSig: { metadata: listing },
    } = ratingData;
    const {
      peerID, inDetail, bigPadding, onPress, isForStore,
    } = this.props;
    const borderStyle = { borderBottomWidth: inDetail ? 1 : 0, borderColor: '#c8c7c7' };
    return (
      <TouchableWithoutFeedback onPress={onPress}>
        <View style={[styles.wrapper, borderStyle, bigPadding ? { padding: 16 } : {}]}>
          {isForStore ? (
            <ListingReview
              peerID={peerID}
              profile={profile}
              review={review}
              overall={overall}
              listing={listing}
              past={timeSince(new Date(seconds * 1000))}
            />
          ) : (
            <BuyerReview
              profile={profile}
              review={review}
              overall={overall}
              past={timeSince(new Date(seconds * 1000))}
              inDetail={inDetail}
            />
          )}
          {inDetail && (
            <View style={styles.scoreWrapper}>
              <Rating title={I18n.t('components.molecules.ProductRating.Quality')} value={quality} />
              <Rating title={I18n.t('components.molecules.ProductRating.as_advertised')} value={description} />
              <Rating title={I18n.t('components.molecules.ProductRating.Delivery')} value={deliverySpeed} />
              <Rating title={I18n.t('components.molecules.ProductRating.Service')} value={customerService} />
            </View>
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  username: state.username,
  password: state.password,
});

export default connect(mapStateToProps)(ProductRating);
