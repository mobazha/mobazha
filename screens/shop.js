/* eslint-disable react/sort-comp */
import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, RefreshControl, FlatList, Platform } from 'react-native';
import { ScrollView, withNavigationFocus, NavigationEvents } from 'react-navigation';
import * as _ from 'lodash';

import MainSlider from '../components/molecules/MainSlider';
import Section from '../components/molecules/Section';
import CategoryList from '../components/templates/CategoryList';
import SearchHeader from '../components/organism/SearchHeader';
import ProductGrid from '../components/organism/ProductGrid';
import ShopGrid from '../components/organism/ShopGrid';
import { screenWrapper } from '../utils/styles';
import Button from '../components/atoms/FullButton';

import {
  fetchTrendingListing,
  fetchFeaturedListing,
  fetchBestsellersListing,
  fetchGamingListing,
  fetchMunchiesListing,
  fetchArtsListing,
  fetchDevicesListing,
  fetchListingsForCategories,
} from '../reducers/product';
import { fetchFeatured } from '../reducers/featured';
import { fetchPromo } from '../reducers/promo';
import { clearFilter } from '../reducers/search';
import { eventTracker } from '../utils/EventTracker';
import { getCategoryItem } from '../config/categories';
import CategoryProductGrid from '../components/templates/CategoryProductGrid';
import CovidModal from '../components/templates/CovidModal';
import EULAModal from '../components/templates/EULAModal';

import { PREVIEWING_CATEGORIES } from '../utils/listings';

import { setEULAPopupSeen } from '../reducers/appstate';

import {I18n} from '../langs/I18n';

const styles = {
  buttonText: {
    fontSize: 13,
    fontWeight: 'bold',
  },
  buttonWrapper: {
    backgroundColor: '#ff3b30',
    borderRadius: 5,
    marginTop: 10,
    marginBottom: 0,
    marginLeft: 8,
    marginRight: 8,
    height: 43,
  },
};


class ShopScreen extends PureComponent {
  state = {
    showQRButton: true,
    refreshing: false,
    promoData: [],
    showCovid: false,
    showEULA: false,
  };

  componentDidMount() {
    this.handleAnonymousFetch();
  }

  setPromo = () => {
    const promoData = _.get(this.props, 'promo.data', []);
    if (promoData.length === 0) {
      this.setState({ promoData: [] });
      return;
    }

    const firstPromo = promoData[0];
    const secondaryPromos = promoData.slice(1, promoData.length);
    const arryCount = Math.min(3, secondaryPromos.length);
    const randomPromos = _.sampleSize(secondaryPromos, arryCount);
    this.setState({
      promoData: [
        firstPromo,
        ...randomPromos,
      ],
    });
  }

  getFeaturedStores = () => {
    const { featured, profiles } = this.props;
    const storeList = _.get(featured, 'data', []);
    const newArray = _.filter(storeList, p => profiles[p]);
    return newArray;
  };

  onRefresh = () => {
    const {
      fetchTrendingListing,
      fetchFeaturedListing,
      fetchBestsellersListing,
      fetchGamingListing,
      fetchMunchiesListing,
      fetchArtsListing,
      fetchDevicesListing,
      fetchFeatured,
      fetchListingsForCategories,
    } = this.props;
    this.setState({ refreshing: true }, () => {
      setTimeout(() => { this.setState({ refreshing: false }); }, 500);
    });
    fetchFeatured();
    fetchTrendingListing();
    fetchFeaturedListing();
    fetchBestsellersListing();
    fetchGamingListing();
    fetchMunchiesListing();
    fetchArtsListing();
    fetchDevicesListing();
    fetchListingsForCategories();
  };

  handleAnonymousFetch = () => {
    const {
      fetchListingsForCategories,
      fetchFeaturedListing,
      fetchTrendingListing,
      fetchBestsellersListing,
      fetchGamingListing,
      fetchMunchiesListing,
      fetchArtsListing,
      fetchDevicesListing,
      fetchFeatured,
      fetchPromo,
    } = this.props;
    fetchPromo();
    fetchFeaturedListing();
    fetchTrendingListing();
    fetchBestsellersListing();
    fetchGamingListing();
    fetchMunchiesListing();
    fetchArtsListing();
    fetchDevicesListing();
    fetchFeatured();
    fetchListingsForCategories();
    this.setPromo();
  }

  handleGoToCategoryOverview = categoryName => () => {
    const item = getCategoryItem(categoryName);
    this.props.navigation.navigate('CategoryOverview', { item });
  }

  handleGoToSettings = () => {
    this.props.navigation.navigate('Settings');
  }

  handleTapSearch = () => {
    this.props.clearFilter();
    this.props.navigation.navigate('SearchResult');
  }

  handleCloseCovid = () => {
    this.setState({ showCovid: false });
  }

  handleCloseEULA = () => {
    this.setState({ showEULA: false });
  }

  handleCreateListing = () => {
    this.setState({ showCovid: false }, () => {
      this.props.navigation.navigate('CreateListing');
    });
  }

  handleShowCovid = () => {
    this.setState({ showCovid: true });
  }

  handleFocus = () => {
    const { EULAPopupSeen, setEULAPopupSeen } = this.props;
    if (!EULAPopupSeen) {
      this.setState({ showEULA: true });
      setEULAPopupSeen(true);
    }
  }

  toListingDetails = source => (params) => {
    eventTracker.trackEvent(`Discover-TappedListing-${source}`, { reference: source });
    this.props.navigation.navigate('Listing', params);
  }

  toExternalStore = (peerID) => {
    eventTracker.trackEvent('Discover-TappedUser');
    this.props.navigation.navigate('ExternalStore', { peerID });
  }

  filterBlocked = (listingInfo) => {
    const { blockedNodes } = this.props;
    return !blockedNodes.includes(_.get(listingInfo, 'relationships.vendor.data.peerID'));
  }

  keyExtractor = item => `category_preview_${item.shortName}`

  renderCategoryPreview = ({ item }) => (
    <CategoryProductGrid
      title={item.title}
      name={item.categoryName}
      productCount={8}
      onPress={this.handleGoToCategoryOverview(item.categoryName)}
    />
  )

  render() {
    const {
      featuredListing = {},
      trendingListing = {},
      bestsellersListing = {},
      gamingListing = {},
      munchiesListing = {},
      devicesListing = {},
      navigation,
    } = this.props;
    const { promoData, refreshing, showCovid, showEULA } = this.state;
    const featuredListings = _.take((featuredListing.data || []).filter(this.filterBlocked), 6);
    const trendingListings = _.take((trendingListing.data || []).filter(this.filterBlocked), 3);
    const bestsellersListings = _.take((bestsellersListing.data || []).filter(this.filterBlocked), 9);
    const gamingListings = _.take((gamingListing.data || []).filter(this.filterBlocked), 9);
    const munchiesListings = _.take((munchiesListing.data || []).filter(this.filterBlocked), 9);
    const devicesListings = _.take((devicesListing.data || []).filter(this.filterBlocked), 9);

    const { showQRButton } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <NavigationEvents onDidFocus={this.handleFocus} />
        <SearchHeader
          onRight={this.handleGoToSettings}
          onClick={this.handleTapSearch}
          navigation={navigation}
          showQRButton={showQRButton}
        />
        <ScrollView
          scrollEventThrottle={400}
          refreshControl={
            <RefreshControl
              refreshing={(featuredListing.loading || trendingListing.loading) && refreshing}
              onRefresh={this.onRefresh}
            />
          }
        >
          {Platform.OS === 'android' && false && (
            <Button
              title="Essential supplies needed (COVID-19)"
              textStyle={styles.buttonText}
              wrapperStyle={styles.buttonWrapper}
              onPress={this.handleShowCovid}
            />
          )}
          <MainSlider items={promoData} />
          <CategoryList />
          {/* <Section title={I18n.t('screens.shop.Trending')}>
            <ProductGrid
              compact
              products={trendingListings}
              keyPrefix="trending"
              toListingDetails={this.toListingDetails('Latest')}
            />
          </Section>
          <Section title={I18n.t('screens.shop.Featured_stores')}>
            <ShopGrid
              shops={this.getFeaturedStores()}
              onPress={this.toExternalStore}
              count={4}
            />
          </Section>
          <Section title={I18n.t('screens.shop.Featured_listings')}>
            <ProductGrid
              compact
              products={featuredListings}
              keyPrefix="featured"
              toListingDetails={this.toListingDetails('Hot')}
            />
          </Section>
          <Section title={I18n.t('screens.shop.Best_Sellers')}>
            <ProductGrid
              compact
              products={bestsellersListings}
              keyPrefix="bestsellers"
              toListingDetails={this.toListingDetails('Bestsellers')}
            />
          </Section>
          <Section title={I18n.t('screens.shop.Gaming')}>
            <ProductGrid
              compact
              products={gamingListings}
              keyPrefix="gaming"
              toListingDetails={this.toListingDetails('Gaming')}
            />
          </Section>
          <Section title={I18n.t('screens.shop.Munchies')}>
            <ProductGrid
              compact
              products={munchiesListings}
              keyPrefix="munchies"
              toListingDetails={this.toListingDetails('Munchies')}
            />
          </Section>
          <Section title={I18n.t('screens.shop.Devices')}>
            <ProductGrid
              compact
              products={devicesListings}
              keyPrefix="devices"
              toListingDetails={this.toListingDetails('Devices')}
            />
          </Section> */}
          <Section title={I18n.t('screens.shop.Best_Sellers')}>
            <ProductGrid
              compact
              products={bestsellersListings}
              keyPrefix="bestsellers"
              toListingDetails={this.toListingDetails('Bestsellers')}
            />
          </Section>

          <FlatList
            data={PREVIEWING_CATEGORIES}
            keyExtractor={this.keyExtractor}
            renderItem={this.renderCategoryPreview}
          />
        </ScrollView>
        <CovidModal
          show={showCovid}
          onClose={this.handleCloseCovid}
          onCreateListing={this.handleCreateListing}
          navigation={navigation}
        />
        <EULAModal
          show={showEULA}
          onClose={this.handleCloseEULA}
          navigation={navigation}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  trendingListing: state.product.trending,
  featuredListing: state.product.featured,
  bestsellersListing: state.product.bestsellers,
  gamingListing: state.product.gaming,
  munchiesListing: state.product.munchies,
  devicesListing: state.product.devices,
  wishlist10: state.wishlist.length > 10 ? state.wishlist.slice(0, 10) : state.wishlist,
  featured: state.featured,
  profiles: state.profiles,
  promo: state.promo,
  blockedNodes: state.settings.blockedNodes,
  EULAPopupSeen: state.appstate.EULAPopupSeen,
});

const mapDispatchToProps = {
  fetchFeaturedListing,
  fetchTrendingListing,
  fetchBestsellersListing,
  fetchGamingListing,
  fetchMunchiesListing,
  fetchArtsListing,
  fetchDevicesListing,
  fetchFeatured,
  fetchListingsForCategories,
  fetchPromo,
  clearFilter,
  setEULAPopupSeen,
};

export default withNavigationFocus(connect(
  mapStateToProps,
  mapDispatchToProps,
)(ShopScreen));
