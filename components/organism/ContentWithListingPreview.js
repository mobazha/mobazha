import React from 'react';
import _ from 'lodash';
import decode from 'unescape';
import ParsedText from 'react-native-parsed-text';

import { getOBParsePatterns } from '../../utils/parseText';
import { getNavRouteAndParamsFromURL } from '../../utils/navigation';
import ListingPreview from '../molecules/ListingPreview';

import { handleOBDeeplinkWithNavigation, createFeedUrlFromPeerIDAndSlug } from '../../utils/navigation';

export function getListingIfExists() {
  const { listings } = this.props;
  const { listingSlug: slug, listingPeerID: peerID } = this.state || {};
  if (peerID) {
    const listing = listings.find(item => (
      _.get(item, 'listing.slug') === slug && _.get(item, 'listing.vendorID.peerID') === peerID
    ));

    if (!_.isEmpty(listing)) {
      return listing;
    }
  }

  return null;
}

export function getListingLinkFromContent(status) {
  const urls = status.match(/\bhttps?:\/\/\S+/gi);
  if (urls && urls.length === 1) {
    const { route, params } = getNavRouteAndParamsFromURL(urls[0]) || {};
    const { slug, peerID } = params || {};

    if (route === 'Listing') {
      return {
        slug,
        peerID,
        displayingContent: status.slice(0, -urls[0].length - 1),
      };
    }
  }

  return { displayingContent: status };
}

export function fetchListingLink() {
  const { fetchListing } = this.props;
  const status = this.getStatusToDecode();

  // fetch Listing link
  const decodedStatus = decode(status);
  const { peerID, slug, displayingContent } = this.getListingLinkFromContent(decodedStatus);
  if (peerID && slug) {
    fetchListing({ slug, peerID });
    this.setState({ displayingContent, listingSlug: slug, listingPeerID: peerID });
  } else {
    this.setState({ displayingContent: decodedStatus });
  }
}

export function handlePressListing() {
  const listing = this.getListingIfExists();
  if (!listing) {
    return;
  }

  const peerID = _.get(listing, 'listing.vendorID.peerID');
  const slug = _.get(listing, 'listing.slug');
  this.props.navigation.push('Listing', { slug, peerID });
}

export function renderContentWithListingPreview(isInFeedPreview = false) {
  const { displayingContent } = this.state;
  const listing = this.getListingIfExists();
  const listingImages = _.get(listing, 'listing.item.images');

  return (
    <React.Fragment>
      {!_.isEmpty(displayingContent) && (
        <ParsedText
          style={this.styles.text}
          parse={getOBParsePatterns(
            this.styles.linkText,
            this.handleDeeplinkPress,
            this.handleCrypoLink,
            this.handleHashtag,
          )}
        >
          {displayingContent}
        </ParsedText>
      )}
      {listing && (
        <ListingPreview
          title={decode(_.get(listing, 'listing.item.title'))}
          thumbnail={listingImages && listingImages[0].small}
          currencyCode={_.get(listing, 'listing.metadata.pricingCurrency')}
          amount={_.get(listing, 'listing.item.price')}
          onPress={this.handlePressListing}
          isInFeedPreview={isInFeedPreview}
        />
      )}
    </React.Fragment>
  );
}

export function handleDeeplinkPress(deeplink) {
  const { navigation } = this.props;
  handleOBDeeplinkWithNavigation(deeplink, navigation);
}

export function handleCrypoLink(coin) {
  return (url => this.props.navigation.navigate('SendMoney', { coin, address: url }));
}

export function handleHashtag(hashtag) {
  this.props.navigation.navigate('Hashtag', { hashtag });
}

export function getFeedInfo(propsFromParam) {
  const props = propsFromParam || this.props;
  const { activityId, getActivity } = props;
  return getActivity(activityId);
}

export function getStatusToDecode(propsFromParam) {
  const props = propsFromParam || this.props;
  let status;
  const item = this.getFeedInfo(props);
  ({ status } = _.get(item, 'object.data.post', {}));

  if (this.isRepost && this.isRepost(props) && status === '') {
    const originActivity = this.getRepostFeedInfo(props);
    const originPost = _.get(originActivity, 'object.data.post', {});
    ({ status } = originPost || {});
  }

  return status;
}
