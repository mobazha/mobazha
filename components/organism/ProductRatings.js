import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Text, TouchableWithoutFeedback, View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { withNavigation } from 'react-navigation';
import { get } from 'lodash';

import { starRatingColor, foregroundColor, primaryTextColor, secondaryTextColor } from '../commonColors';

import ProductSection from '../atoms/ProductSection';
import ProductRating from '../molecules/ProductRating';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  showAllButton: {
    width: '100%',
    paddingHorizontal: 17,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
    marginTop: 12,
  },
  showAllText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  noReviews: {
    fontStyle: 'italic',
    color: secondaryTextColor,
  },
  count: {
    textDecorationLine: 'underline',
  },
};

class ProductRatings extends PureComponent {
  onShow = () => {
    const { peerID, slug, navigation } = this.props;
    eventTracker.trackEvent('ListingSectionTapped-SeeAllReviews');
    navigation.push('ProductRatings', { peerID, slug });
  };

  getRatings = () => {
    const { ratings, peerID, slug } = this.props;
    return get(ratings, `${peerID}/${slug}`, {});
  }

  render() {
    const { ratings = [], avgRating = 0, count = 0 } = this.getRatings();

    if (!ratings) {
      return null;
    }

    const nodes = [];
    for (let i = 0; i < Math.min(3, ratings.length); i += 1) {
      nodes.push(<ProductRating
        nodeID={ratings[i]}
        key={i}
        onPress={this.onShow}
      />);
    }
    return (
      <ProductSection
        title={I18n.t('components.organism.ProductRatings.reviews')}
        subTitle={
          <Text>
            &nbsp;
            <Ionicons name="md-star" size={16} color={starRatingColor} />
            {`${avgRating.toFixed(1)} `}
            (<Text style={styles.count}>{count}</Text>)
          </Text>
        }
        actionTitle={count > 3 ? 'View All' : ''}
        onAction={this.onShow}
      >
        {ratings.length > 0 ? (
          <View>
            {nodes}
            <TouchableWithoutFeedback onPress={this.onShow}>
              <View style={styles.showAllButton}>
                <Text style={styles.showAllText}>{I18n.t('components.organism.ProductRatings.see_all_reviews', {length:ratings.length})}</Text>
              </View>
            </TouchableWithoutFeedback>
          </View>
        ) : (
          <Text style={styles.noReviews}>{I18n.t('components.organism.ProductRatings.no_reviews_yet')}</Text>
        )}
      </ProductSection>
    );
  }
}

const mapStateToProps = state => ({ ratings: state.ratings.data });

export default withNavigation(connect(mapStateToProps)(ProductRatings));
