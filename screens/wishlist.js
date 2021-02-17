import React, { PureComponent } from 'react';
import { View } from 'react-native';
import { connect } from 'react-redux';

import Header from '../components/molecules/Header';
import WishListView from '../components/templates/wishlist';
import NavBackButton from '../components/atoms/NavBackButton';
import { foregroundColor } from '../components/commonColors';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: foregroundColor,
  },
};

class WishList extends PureComponent {
  handleGoBack = () => this.props.navigation.goBack()
  handleGoToListing = (params) => {
    eventTracker.trackEvent('Wishlist-TappedWishlistItem');
    this.props.navigation.push('Listing', params);
  }

  render() {
    const { wishlist } = this.props;
    return (
      <View style={styles.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} title={I18n.t('screens.wishlist.Wishlist')} />
        <WishListView
          results={wishlist}
          toListingDetails={this.handleGoToListing}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  wishlist: state.wishlist,
});

export default connect(mapStateToProps)(WishList);
