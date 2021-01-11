import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Text, TouchableWithoutFeedback, View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { withNavigation } from 'react-navigation';
import { get } from 'lodash';

import { starRatingColor, foregroundColor, primaryTextColor, secondaryTextColor } from '../commonColors';

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
  avgRating: {
    color: '#8a8a8f',
  },
  count: {
    color: '#8a8a8f',
    textDecorationLine: 'underline',
  },
};

class AverageRating extends PureComponent {
  onShow = () => {
    const { peerID, slug, navigation } = this.props;
    navigation.push('ProductRatings', { peerID, slug });
  };

  getRatings = () => {
    const { ratings, peerID, slug } = this.props;
    return get(ratings, `${peerID}/${slug}`, {});
  }

  render() {
    const { ratings = [], avgRating = 0, count = 0 } = this.getRatings();

    if (!ratings) {
      return <Text style={styles.noReviews}>...</Text>;
    }

    return (
      <TouchableWithoutFeedback onPress={this.onShow}>
        <View>
          {ratings.length > 0 ? (
            <Text>
              {[1, 2, 3, 4, 5].map(index => (index <= Math.floor(avgRating) ? (
                <Ionicons key={index} name="md-star" size={16} color={starRatingColor} />
              ) : (
                <Ionicons key={index} name="md-star" size={16} color="#c8c7cc" />
              )))}
              &nbsp;
              <Text style={styles.avgRating}>{`${avgRating.toFixed(1)} (`}</Text>
              <Text style={styles.count}>{count}</Text>
              <Text style={styles.avgRating}>)</Text>
            </Text>
          ) : (
            <Text style={styles.noReviews}>{I18n.t('components.organism.AverageRating.no_reviews')}</Text>
          )}
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({ ratings: state.ratings.data });

export default withNavigation(connect(mapStateToProps)(AverageRating));
