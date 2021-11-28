import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import {
  Image, StyleSheet, View, ScrollView, Text, ActivityIndicator, Dimensions, Alert,
} from 'react-native';
import * as _ from 'lodash';
import ImageViewer from 'react-native-image-zoom-viewer';
import { ifIphoneX } from 'react-native-iphone-x-helper';
import Analytics from 'appcenter-analytics';

import SellerInfo from '../components/molecules/SellerInfo';
import ProductImageSlider from '../components/molecules/ProductImageSlider';
import ProductName from '../components/atoms/ProductName';
import ProductPrice from '../components/atoms/ProductPrice';
import ProductState from '../components/atoms/ProductState';
import ModalBackButton from '../components/atoms/ModalBackButton';
import ProductDescription from '../components/molecules/ProductDescription';
import ProductPolicy from '../components/molecules/ProductPolicy';
import ProductTags from '../components/molecules/ProductTags';
import ListingHeader from '../components/molecules/ListingHeader';
import VendorListings from '../components/organism/VendorListings';
import { OBDarkModal } from '../components/templates/OBModal';
import { secondaryTextColor, formLabelColor } from '../components/commonColors';
import ProductRatings from '../components/organism/ProductRatings';
import ListingPaymentOptions from '../components/molecules/ListingPaymentOptions';
import HollowButton from '../components/atoms/HollowButton';
import OBActionSheet from '../components/organism/ActionSheet';
import { StatusBarSpacer } from '../status-bar';
import ReportTemplate from '../components/templates/ReportTemplate';

import { fetchListing } from '../reducers/listings';
import { fetchRatings } from '../reducers/ratings';
import { blockNode } from '../reducers/settings';
import { editListing, deleteListing } from '../reducers/createListing';
import { addWishListing, removeWishListing } from '../reducers/wishlist';
import { setKeyword, setShippingOption, clearShippingOption, showPanel } from '../reducers/appstate';
import { createListingUrlFromPeerIDAndSlug } from '../utils/navigation';
import { getImageSourceForImageViewer } from '../utils/files';
import { initShippingOptionWithMinShippingPrice } from '../utils/stockManage';

import { reportListing } from '../api/products';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';

const { width: SCREEN_WIDTH, height: SCREEN_HEIGHT } = Dimensions.get('window');
const ListingFailImg = require('../assets/images/listing_fail.png');

const styles = {
  wrapper: {
    flex: 1,
    backgroundColor: 'white',
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 220,
    top: (SCREEN_WIDTH / 2) - 20,
  },
  overlayNarrow: {
    width: 120,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
  shopTitle: {
    paddingHorizontal: 20,
    paddingVertical: 4,
    fontSize: 14,
    fontStyle: 'normal',
    lineHeight: 20,
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  underlineTitle: {
    textDecorationLine: 'underline',
  },
  fullWrapper: {
    ...StyleSheet.absoluteFill,
    height: SCREEN_HEIGHT,
    alignItems: 'center',
    justifyContent: 'center',
  },
  scrollView: {
    flex: 1,
    backgroundColor: 'white',
  },
  bottomWrapper: {
    paddingBottom: ifIphoneX(34, 0),
  },
  emptyWrapper: {
    flex: 1,
    backgroundColor: '#ffffff',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 185,
  },
  emptyText: {
    color: formLabelColor,
    fontSize: 15,
    textAlign: 'center',
    marginVertical: 10,
  },
  listingFailImg: {
    tintColor: formLabelColor,
    width: 53,
    height: 48,
  },
};

class Listing extends PureComponent {
  static getDerivedStateFromProps(props) {
    const {
      listings, navigation, shippingAddress, settings: { shippingAddresses },
    } = props;
    const { slug, peerID, hash } = navigation.state.params;
    const listingInfo = listings.find((item) => {
      if (hash) {
        return item.hash === hash;
      }
      if (peerID) {
        return item.listing.slug === slug && _.get(item, 'listing.vendorID.peerID') === peerID;
      }
      return item.listing.slug === slug;
    });

    if (_.isEmpty(listingInfo)) {
      return {};
    }

    const { metadata: { contractType }, shippingOptions } = listingInfo.listing;
    if (contractType !== 'PHYSICAL_GOOD' || !shippingOptions) {
      return { listingInfo };
    }

    let country;
    if (!_.isEmpty(shippingAddresses)) {
      country = shippingAddresses[shippingAddress].country;
    }

    return {
      listingInfo,
      shippingOptions: _.isEmpty(shippingAddresses) ? (
        []
      ) : (
        shippingOptions && shippingOptions.filter(op => op.regions.includes('ALL') || op.regions.includes(country))
      ),
    };
  }

  state = {
    imagesForModal: [],
    liked: false,
    unliked: false,
    initialIndex: 0,
  };

  isDueTimerHandle = null;

  componentDidMount() {
    this.initShippingOption = initShippingOptionWithMinShippingPrice.bind(this);
    this.loadListing();

    setTimeout(this.loadListingFromGateway, 5000);
    this.isDueTimerHandle = setTimeout(() => this.setState({ isDue: true }), 10000);


    this.initShippingOption();
  }

  componentDidUpdate(prevProps, prevState) {
    if (!prevState.listingInfo && this.state.listingInfo) {
      this.initShippingOption();
    }
  }

  onLike = (wishlist) => {
    const { addWishListing, removeWishListing } = this.props;
    const { listingInfo } = this.state;
    if (wishlist) {
      addWishListing(listingInfo);

      this.setState({ liked: true });
      setTimeout(() => this.setState({ liked: false }), 2000);
    } else {
      removeWishListing(listingInfo);

      this.setState({ unliked: true });
      setTimeout(() => this.setState({ unliked: false }), 2000);
    }
  };

  onPressTag = (tag) => {
    eventTracker.trackEvent('ListingSectionTapped-Tag');
    this.props.setKeyword(tag);
    this.props.navigation.navigate('SearchResult', { isFromTag: true });
  };

  onPressImage = (initialIndex) => {
    const { listingInfo } = this.state;
    const images = _.get(listingInfo, 'listing.item.images');
    const imagesForModal = images.map(val => getImageSourceForImageViewer(val.large));
    this.setState({ imagesForModal, initialIndex });
    eventTracker.trackEvent('ListingSectionTapped-Photo');
  };

  onCloseImageModal = () => {
    this.setState({ imagesForModal: [], initialIndex: 0 });
  };

  getActionSheetItems = () => {
    const { myPeerID } = this.props;
    const { listingInfo } = this.state;
    const itemPeerID = _.get(listingInfo, 'listing.vendorID.peerID');
    if (myPeerID !== itemPeerID) {
      return {
        options: [I18n.t('screens.listing.report_listing'), I18n.t('screens.listing.block_user'), I18n.t('screens.listing.cancel')],
        cancelButtonIndex: 2,
      };
    }
    return {
      options: [I18n.t('screens.listing.Edit_Listing'), I18n.t('screens.listing.Delete_Listing'), I18n.t('screens.listing.cancel')],
      cancelButtonIndex: 2,
    };
  }

  setActionSheet = (component) => { this.actionSheet = component; };

  setReportTemplateRef = (component) => { this.reportTemplate = component; }

  loadListing = () => {
    const { fetchListing, fetchRatings, navigation } = this.props;
    const { hash, slug, peerID } = navigation.state.params;
    if (hash) {
      fetchListing({ hash, peerID, slug });
    } else {
      fetchListing({ slug, peerID });
    }

    fetchRatings({ peerID, slug });
  }

  loadListingFromGateway = () => {
    const { listingInfo } = this.state;
    if (!listingInfo) {
      const { fetchListing, navigation } = this.props;
      const { hash, slug, peerID } = navigation.state.params;
      if (hash) {
        fetchListing({ slug, peerID });
      } else {
        if (this.isDueTimerHandle) {
          clearTimeout(this.isDueTimerHandle);
          this.setState({ isDue: true });
        }
      }
    }
  }

  submitReport = (reason, option) => {
    const { listingInfo } = this.state;
    const itemPeerID = _.get(listingInfo, 'listing.vendorID.peerID');
    const slug = _.get(listingInfo, 'listing.slug');
    const reportReason = `${reason}-${option}`;
    reportListing(itemPeerID, slug, reportReason);
    this.handleReport();
  }

  handleReport = () => {
    this.setState({ reported: true });
    setTimeout(() => this.setState({ reported: false }), 2000);
  };

  handleChange = (index) => {
    const { myPeerID } = this.props;
    const { listingInfo } = this.state;
    const itemPeerID = _.get(listingInfo, 'listing.vendorID.peerID');
    if (myPeerID !== itemPeerID) {
      switch (index) {
        case 0: {
          if (this.reportTemplate) {
            this.reportTemplate.startStep();
          }
          break;
        }
        case 1: {
          Alert.alert(I18n.t('screens.listing.are_you_sure'), I18n.t('screens.listing.ask_block'), [
            { text: I18n.t('screens.listing.cancel')},
            { text: 'OK', onPress: this.blockNode },
          ]);
          break;
        }
        default:
          break;
      }
    } else {
      switch (index) {
        case 0: {
          this.handleEdit();
          break;
        }
        case 1: {
          this.handleDelete();
          break;
        }
        default:
          break;
      }
    }
  };

  handleEdit = () => {
    const { listingInfo } = this.state;
    const updateListings = this.props.navigation.getParam('updateListings');
    this.props.editListing({ listingInfo });
    this.props.navigation.navigate({
      routeName: 'EditListing',
      params: {
        updateListings,
      },
    });
  }

  handleShare = () => {
    const { showPanel } = this.props;
    const { listingInfo } = this.state;
    const { listing } = listingInfo;

    showPanel({ type: 'share', panelData: listing });
  };

  handleDelete = () => {
    Alert.alert(I18n.t('screens.listing.ask_delete'), I18n.t('screens.listing.delete_hint'), [
      { text: I18n.t('screens.listing.cancel')},
      {
        text: I18n.t('screens.listing.remove'),
        onPress: this.deleteListing,
      },
    ]);
  }

  handleBuy = () => {
    const { listingInfo } = this.state;
    const options = _.get(listingInfo, 'listing.item.options');
    if (_.isEmpty(options)) {
      this.props.navigation.navigate({
        routeName: 'CheckoutListing',
        params: { listing: listingInfo, quantity: 1 },
      });
    } else {
      this.props.navigation.navigate({
        routeName: 'CheckoutOption',
        params: { listing: listingInfo },
      });
    }
    eventTracker.trackEvent('ListingSectionTapped-BuyNow');
  }

  handlePressMore = () => {
    this.actionSheet.show();
  }

  handleToStore = () => {
    const { peerID, hash } = this.props.navigation.state.params;
    const { listingInfo } = this.state;
    eventTracker.trackEvent('ListingSectionTapped-VisitStore');
    if (peerID) {
      this.props.navigation.navigate('ExternalStore', { peerID });
    } else if (hash) {
      const peerID = _.get(listingInfo, 'listing.vendorID.peerID');
      this.props.navigation.navigate('ExternalStore', { peerID });
    } else {
      this.props.navigation.navigate('Store');
    }
  }

  handleToMessage = () => {
    const { navigation } = this.props;
    const peerID = navigation.getParam('peerID');
    const { listingInfo } = this.state;
    const vendorID = _.get(listingInfo, 'listing.vendorID.peerID');
    const slug = _.get(listingInfo, 'listing.slug');
    const listingUrl = `${createListingUrlFromPeerIDAndSlug(vendorID, slug)} `;
    navigation.navigate('ChatDetail', { peerID: peerID || vendorID, subject: '', listingUrl });
  }

  handleGoBack = () => {
    const { navigation } = this.props;
    const screenKey = navigation.getParam('screenKey');
    if (screenKey) {
      navigation.pop(2);
    } else {
      navigation.goBack();
    }
  }

  handleRetry = () => {
    this.setState({ isDue: false });
    this.loadListing();
    setTimeout(() => this.setState({ isDue: true }), 5000);
  }

  blockNode = () => {
    const { blockNode, navigation } = this.props;
    const { listingInfo } = this.state;
    const itemPeerID = _.get(listingInfo, 'listing.vendorID.peerID');
    blockNode(itemPeerID);
    navigation.pop();
  }

  deleteListing = () => {
    const slug = this.props.navigation.getParam('slug');
    this.props.deleteListing({ slug });
    this.props.navigation.goBack();
  }

  toListingDetails = (params) => {
    eventTracker.trackEvent('ListingSectionTapped-RelatedListing');
    this.props.navigation.push('Listing', params);
  }

  toExternalStore = (peerID) => {
    this.props.navigation.push('ExternalStore', { peerID });
  }

  trackSwipe = () => {
    eventTracker.trackEvent('Listing/SwipedPhotos');
  }

  renderDueState = () => (
    <View style={styles.fullWrapper}>
      <Image style={styles.listingFailImg} source={ListingFailImg} />
      <Text style={styles.emptyText}>{I18n.t('screens.listing.failed_load')}.</Text>
      <HollowButton title={I18n.t('screens.listing.retry')} onPress={this.handleRetry} />
    </View>
  );

  renderLoadingState = () => (
    <View style={styles.fullWrapper}>
      <ActivityIndicator size="large" color={formLabelColor} />
      <Text style={styles.emptyText}>{I18n.t('screens.listing.loading')}</Text>
    </View>
  );

  renderLoadingImageViewer = () => <ActivityIndicator style={styles.fullWrapper} size="large" color={formLabelColor} />

  render() {
    const { wishlist, shippingOption } = this.props;
    const {
      imagesForModal, liked, unliked, initialIndex, isDue, reported,
    } = this.state;
    const { peerID } = this.props.navigation.state.params;
    const { listingInfo } = this.state;

    if (!listingInfo) {
      return (
        <View style={styles.wrapper}>
          <ListingHeader onBack={this.handleGoBack} />
          {isDue ? (
            this.renderDueState()
          ) : (
            this.renderLoadingState()
          )}
        </View>
      );
    }

    const { listing } = listingInfo;
    const {
      item: {
        images,
        title,
        condition,
        description,
        tags,
      },
      slug: listingSlug,
      termsAndConditions,
      refundPolicy,
      vendorID: {
        peerID: vendorPeerID,
      },
      metadata: {
        contractType,
      },
    } = listing;
    const isWishList = wishlist.findIndex(item => _.get(item, 'listing.slug') === _.get(listing, 'slug')) >= 0;

    return (
      <View style={styles.wrapper}>
        <ListingHeader
          showActions
          onBack={this.handleGoBack}
          wishlist={isWishList}
          action={this.onLike}
          onMore={this.handlePressMore}
          onShare={this.handleShare}
        />
        <StatusBarSpacer />
        <ScrollView style={styles.scrollView}>
          <ProductImageSlider
            images={images}
            onShowImage={this.onPressImage}
          />
          <ProductName prodName={title} />
          <ProductState
            type={contractType || ''}
            condition={condition || ''}
          />
          <ProductDescription description={description} peerID={vendorPeerID} slug={listingSlug} />
          <ProductRatings peerID={vendorPeerID} slug={listingSlug} />
          <ListingPaymentOptions listing={listing} />
          <ProductPolicy
            policy={I18n.t('screens.listing.policy1')}
            content={refundPolicy}
            peerID={vendorPeerID}
            slug={listingSlug}
          />
          <ProductPolicy
            policy={I18n.t('screens.listing.policy2')}
            content={termsAndConditions}
            peerID={vendorPeerID}
            slug={listingSlug}
          />
          {!_.isEmpty(tags) && (
            <ProductTags tags={tags} onPress={this.onPressTag} />
          )}
          <SellerInfo
            peerID={vendorPeerID}
            toStore={this.handleToStore}
            toMessage={this.handleToMessage}
          />
          <VendorListings
            peerID={vendorPeerID}
            slug={listingSlug}
            toListingDetails={this.toListingDetails}
            toStore={this.toExternalStore}
          />
          <OBDarkModal
            onRequestClose={this.onCloseImageModal}
            visible={imagesForModal.length > 0}
            darkContent
          >
            <ImageViewer
              imageUrls={imagesForModal}
              index={initialIndex}
              onChange={this.trackSwipe}
              enableSwipeDown
              enablePreload
              onCancel={this.onCloseImageModal}
              loadingRender={this.renderLoadingImageViewer}
            />
            <ModalBackButton onPress={this.onCloseImageModal} />
          </OBDarkModal>
        </ScrollView>
        <View style={styles.bottomWrapper}>
          <ProductPrice
            peerID={vendorPeerID}
            slug={listingSlug}
            listing={listing}
            shippingOption={shippingOption}
            onBuy={this.handleBuy}
          />
        </View>
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          {...this.getActionSheetItems()}
        />
        {(liked || unliked || reported) && (
          <View style={[styles.overlay, reported && styles.overlayNarrow]}>
            <Text style={styles.overlayText}>
              {liked ? I18n.t('screens.listing.add_wishlist'): unliked ? I18n.t('screens.listing.remove_wishlist') : I18n.t('screens.listing.reported')}
            </Text>
          </View>
        )}
        <ReportTemplate type="listing" ref={this.setReportTemplateRef} submit={this.submitReport} />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  settings: state.settings,
  shippingOption: state.appstate.shippingOption,
  shippingAddress: state.appstate.shippingAddress,
  listings: state.listings.data,
  wishlist: state.wishlist,
  localCurrency: state.settings.localCurrency,
  rates: state.exchangeRate.rates,
  myPeerID: _.get(state, 'profile.data.peerID'),
});

const mapDispatchToProps = {
  fetchRatings,
  fetchListing,
  addWishListing,
  removeWishListing,
  setKeyword,
  editListing,
  deleteListing,
  setShippingOption,
  clearShippingOption,
  blockNode,
  showPanel,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(Listing);
