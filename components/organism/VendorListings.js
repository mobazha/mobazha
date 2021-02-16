import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { FlatList, Text } from 'react-native';
import * as _ from 'lodash';
import decode from 'unescape';

import ProductCard, { styles as cardStyles } from '../molecules/ProductCard';

import { getListings } from '../../api/products';
import { shuffle } from '../../utils/listings';

class VendorListings extends PureComponent {
  state = {}

  UNSAFE_componentWillMount() {
    const {
      peerID, username, password, slug,
    } = this.props;
    getListings(username, password, peerID).then((response) => {
      if (response.length) {
        const listings = shuffle(response.filter(item => item.slug !== slug)).slice(0, 6);
        this.setState({ listings: listings.length % 2 === 0 ? listings : [...listings, {}] });
      } else {
        this.setState({ listings: [] });
      }
    });
  }

  renderItem = ({ item }) => {
    if (_.isEmpty(item)) {
      return null;
    }
    const { peerID, toListingDetails } = this.props;
    return (
      <ProductCard
        slug={item.slug}
        title={decode(item.title)}
        thumbnail={_.get(item, 'thumbnail.small')}
        averageRating={item.averageRating || 0}
        ratingCount={item.ratingCount || 0}
        currencyCode={_.get(item, 'bigPrice.currencyCode', '')}
        amount={_.get(item, 'bigPrice.amount', 0)}
        peerID={peerID}
        hash={item.hash}
        onPress={toListingDetails}
      />
    );
  }

  render() {
    const { listings } = this.state;
    if (!listings) {
      return null;
    }

    return (
      <FlatList
        numColumns={2}
        columnWrapperStyle={cardStyles.columnWrapperStyle}
        keyExtractor={(val, index) => `infinite_${index}`}
        horizontal={false}
        data={listings}
        renderItem={this.renderItem}
      />
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
});

export default connect(mapStateToProps)(VendorListings);
