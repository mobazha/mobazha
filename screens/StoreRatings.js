import React, { PureComponent } from 'react';
import { View, ScrollView, Text } from 'react-native';
import { connect } from 'react-redux';
import * as _ from 'lodash';

import NavBackButton from '../components/atoms/NavBackButton';
import ProductRating from '../components/molecules/ProductRating';
import Header from '../components/molecules/Header';
import { getRatings } from '../api/products';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
  },
  placeholderText: {
    color: '#8A8A8A',
    textAlign: 'center',
    marginTop: 22,
    fontSize: 15,
  },
};

class StoreRatings extends PureComponent {
  state = {
    ratings: null,
  };

  UNSAFE_componentWillMount() {
    const { username, password, profile } = this.props;
    let peerID = this.props.navigation.getParam('peerID');
    if (!peerID) { peerID = profile.peerID; }
    getRatings(username, password, null, peerID).then((response) => {
      if (response.success !== false) {
        this.setState({ ratings: (response.ratings || []).reverse() });
      }
    });
  }

  handleBack = () => {
    this.props.navigation.goBack();
  }

  render() {
    const { ratings } = this.state;
    const profile = this.props.navigation.getParam('profile');
    const peerID = this.props.navigation.getParam('peerID');

    return (
      <View style={screenWrapper.wrapper}>
        <Header title={I18n.t('screens.StoreRatings.Reviews')} left={<NavBackButton />} onLeft={this.handleBack} />
        <ScrollView style={styles.wrapper}>
          {ratings && ratings.length > 0 && (
            ratings.map((item, idx) => (
              <ProductRating
                peerID={peerID}
                nodeID={item}
                key={idx}
                inDetail
                bigPadding
                isForStore
              />
            ))
          )}
          {ratings && ratings.length === 0 && (
            <Text style={styles.placeholderText}>
              {peerID ? (
                `${_.get(profile, 'name')} ${I18n.t('screens.StoreRatings.no_reviews1')}`
              ) : (
                I18n.t('screens.StoreRatings.no_reviews2')
              )}
            </Text>
          )}
        </ScrollView>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
  profile: state.profile.data,
});

export default connect(mapStateToProps)(StoreRatings);
