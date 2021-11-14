import React, { PureComponent } from 'react';
import { View, Text, ActivityIndicator } from 'react-native';
import { connect } from 'react-redux';
import { withNavigation } from 'react-navigation';
import * as _ from 'lodash';

import InfiniteProducts from '../templates/InfiniteProducts';
import HollowButton from '../atoms/HollowButton';
import Listings from '../organism/Listings';
import { fetchListings } from '../../reducers/storeListings';
import { getListings } from '../../api/products';
import { foregroundColor, brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: foregroundColor,
    paddingVertical: 43,
    borderTopWidth: 0,
  },
  mainText: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#8a8a8f',
    marginBottom: 2,
  },
  description: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
    marginBottom: 10,
  },
  loadingText: {
    marginTop: 20,
  },
};

class ListingsTab extends PureComponent {
  state = {
    listings: [],
    pos: 10,
    loading: true,
  };

  componentDidMount() {
    this.updateListings();
  }

  componentDidUpdate(prevProps) {
    if (!this.props.endReached && prevProps.endReached) {
      this.movePos();
    }
    if (_.get(this.props, 'listings.length') !== _.get(prevProps, 'listings.length')) {
      this.props.onListingCountChange(this.props.listings.length);
    }
    if (this.props.peerID !== prevProps.peerID) {
      this.updateListings();
    }
    if (this.props.listingStatus !== prevProps.listingStatus && this.props.listingStatus === 'success') {
      this.updateListings();
    }
  }

  onFetchListings = (response) => {
    if(response.length) {
      this.setState({ listings: response, pos: 10, loading: false });
      this.props.onListingCountChange(response.length);
    } else {
      this.setState({ listings: [], pos: 10, loading: false });
      this.props.onListingCountChange(0);
    }
  }

  updateListings() {
    const { peerID, username, password } = this.props;
    this.setState({ loading: true });
    if (peerID) {
      // pull only one listing just to check if there is listing for that peerID or not (for now)
      getListings(username, password, peerID).then(this.onFetchListings);
    } else {
      this.props.fetchListings();
      this.props.onListingCountChange(this.props.listings.length);
    }
  }

  createListing = () => {
    const { navigation } = this.props;
    const navKey = navigation.state.key;
    navigation.navigate({
      routeName: 'CreateListing',
      params: { navKey },
    });
  };

  movePos = () => {
    const { pos } = this.state;
    const { peerID } = this.props;
    const { listings } = peerID ? this.state : this.props;
    if (pos < listings.length) {
      this.setState({ pos: pos + 10 });
    }
  };

  handleGoToListingDetails = (params) => {
    this.props.navigation.navigate('Listing', params);
  }

  render() {
    const { peerID, onPress, externalStore, mode, listingStatus } = this.props;
    const { listings } = peerID ? this.state : this.props;
    const loading = peerID && this.state.loading;

    const { pos } = this.state;
    const itemsToRender = _.takeRight(listings, pos);
    if (loading) {
      return (
        <View style={styles.wrapper}>
          <ActivityIndicator size="large" color={brandColor} />
          <Text style={[styles.description, styles.loadingText]}>{I18n.t('components.templates.ListingsTab.loading')}</Text>
        </View>
      );
    }
    if ((listings.results ? listings.results.total : listings.length) === 0 ) {
      if (externalStore) {
        return (
          <View style={styles.wrapper}>
            <Text style={[styles.description, { marginBottom: 2 }]}>
              {I18n.t('components.templates.ListingsTab.no_sale')}              
            </Text>
            <Text style={styles.description}>{I18n.t('components.templates.ListingsTab.check_later')} </Text>
          </View>
        );
      } else {
        return (
          <View style={styles.wrapper}>
            <Text style={styles.mainText}>{I18n.t('components.templates.ListingsTab.store_empty')} </Text>
            <Text style={styles.description}>{I18n.t('components.templates.ListingsTab.put_for_sale')} </Text>
            <HollowButton title={I18n.t('components.templates.ListingsTab.create_listing')} onPress={this.createListing} />
          </View>
        );
      }
    } else {
      // if (externalStore) {
      //   return (
      //     <InfiniteProducts
      //       queryString={`q=*&peerID=${peerID}&nsfw=false&network=mainnet`}
      //       toListingDetails={this.handleGoToListingDetails}
      //       showInitialLoading
      //     />
      //   );
      // } else {
        return (
          <Listings
            mode={mode}
            counts={listings.length}
            results={itemsToRender}
            onPress={onPress}
            peerID={peerID}
            status={listingStatus}
            externalStore={externalStore}
          />
        );
      // }
    }
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
  username: state.appstate.username,
  password: state.appstate.password,
  listings: state.storeListings.listings,
  listingStatus: state.storeListings.status,
});

const mapDispatchToProps = {
  fetchListings,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
  null,
  { withRef: true },
)(ListingsTab));
