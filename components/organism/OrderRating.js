import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text } from 'react-native';
import { isEmpty } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';

import BuyerReview from '../molecules/BuyerReview';
import Rating from '../atoms/Rating';

import { getProfile } from '../../api/profile';
import { formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  reviewWrapper: {
    paddingVertical: 12,
  },
  emptyWrapper: {
    paddingVertical: 12,
    flexDirection: 'row',
  },
  emptyText: {
    fontSize: 15,
    fontStyle: 'italic',
    color: formLabelColor,
  },
  icon: {
    marginRight: 3,
  },
  scoreWrapper: {
    marginLeft: 48,
  },
};

class OrderRating extends PureComponent {
  state = {
    profile: {},
  };

  UNSAFE_componentWillMount() {
    const { buyerID } = this.props;
    getProfile(buyerID).then((response) => {
      this.setState({
        profile: response,
      });
    });
  }

  renderReviews() {
    const { rating, noTopPadding } = this.props;
    const { profile } = this.state;
    const {
      overall, quality, description, deliverySpeed, customerService, review,
    } = rating.ratingData;
    return (
      <View style={[styles.reviewWrapper, noTopPadding && { paddingTop: 0 }]}>
        {[
          (!isEmpty(profile) && <BuyerReview
            review={review}
            profile={profile}
            overall={overall}
          />),
          <View style={!isEmpty(profile) && styles.scoreWrapper}>
            <Rating title="Overall" value={overall} />
            <Rating title="Quality" value={quality} />
            <Rating title="As advertised" value={description} />
            <Rating title="Delivery" value={deliverySpeed} />
            <Rating title="Service" value={customerService} />
          </View>,
        ]}
      </View>
    );
  }

  renderEmpty() {
    const { name } = this.state.profile;
    return (
      <View style={styles.emptyWrapper}>
        <Ionicons name="md-star" size={17} style={styles.icon} color="#f9d553" />
        <Text style={styles.emptyText}>{I18n.t('components.organism.OrderRating.no_feedback', {name:name})}</Text>
      </View>
    );
  }

  render() {
    const { rating } = this.props;
    return isEmpty(rating) ? this.renderEmpty() : this.renderReviews();
  }
}

const mapStateToProps = ({ appstate: { username, password } }) => ({
  username,
  password,
});

export default connect(mapStateToProps)(OrderRating);
