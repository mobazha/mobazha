import React, { PureComponent } from 'react';
import { Text, FlatList, View } from 'react-native';
import { connect } from 'react-redux';
import { withNavigation } from 'react-navigation';

import NavBackButton from '../components/atoms/NavBackButton';
import ProductRating from '../components/molecules/ProductRating';
import Header from '../components/molecules/Header';
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

class ProductRatings extends PureComponent {
  getRatings = () => {
    const { ratings, navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    const slug = navigation.getParam('slug');
    return ratings[`${peerID}/${slug}`] || {};
  }

  keyExtractor = item => `product_rating_${item}`;

  renderRatingItem = ({ item }) => <ProductRating nodeID={item} inDetail bigPadding />

  render() {
    const { ratings = [] } = this.getRatings();
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          title={I18n.t('screens.ProductRatings.reviews')}
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
        />
        <FlatList
          style={styles.wrapper}
          data={ratings}
          renderItem={this.renderRatingItem}
          keyExtractor={this.keyExtractor}
          ListEmptyComponent={<Text style={styles.placeholderText}>{I18n.t('screens.ProductRatings.No_reviews')}</Text>}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({ ratings: state.ratings.data });

export default withNavigation(connect(mapStateToProps)(ProductRatings));
