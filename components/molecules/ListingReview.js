import React from 'react';
import { View, Text, Image, TouchableWithoutFeedback } from 'react-native';
import { isEmpty, get } from 'lodash';
import decode from 'unescape';
import { withNavigation } from 'react-navigation';

import { staticLabelColor } from '../commonColors';
import Stars from '../atoms/Stars';
import { getImageSourceWithDefault } from '../../utils/files';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    paddingBottom: 14,
  },
  topWrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  listingImgBtn: {
    width: 38,
    height: 38,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 10,
    borderRadius: 2,
    borderWidth: 1,
    borderColor: 'rgba(0, 0, 0, 0.2)',
  },
  listingImg: {
    width: '100%',
    height: '100%',
  },
  content: {
    flex: 1,
    flexDirection: 'column',
  },
  listingTitleBtn: {
    flex: 1,
  },
  listingTitle: {
    fontWeight: 'bold',
    fontSize: 15,
    color: 'black',
  },
  past: {
    fontSize: 12,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: staticLabelColor,
  },
  name: {
    fontSize: 12,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: staticLabelColor,
    marginLeft: 4,
  },
  nameContainer: {
    marginTop: 5,
    flexDirection: 'row',
    alignItems: 'center',
  },
  review: {
    width: '100%',
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    color: 'black',
    marginTop: 5,
  },
  noMessage: {
    color: staticLabelColor,
    fontStyle: 'italic',
  },
};

export default withNavigation(({
  peerID, profile, review, overall, style, past, listing, navigation,
}) => {
  const name = get(profile, 'name', 'Anonymous');
  const handleNavigateToListing = () => {
    return navigation.push('Listing', {
      slug: listing.listingSlug, peerID,
    });
  };

  return (
    <View style={[styles.wrapper, style]}>
      <TouchableWithoutFeedback onPress={handleNavigateToListing}>
        <View style={styles.listingImgBtn}>
          <Image
            style={styles.listingImg}
            source={getImageSourceWithDefault(get(listing, 'thumbnail.small'))}
          />
        </View>
      </TouchableWithoutFeedback>
      <View style={styles.content}>
        <View style={styles.topWrapper}>
          <TouchableWithoutFeedback onPress={handleNavigateToListing}>
            <View style={styles.listingTitleBtn}>
              <Text style={styles.listingTitle}>{decode(listing.listingTitle)}</Text>
            </View>
          </TouchableWithoutFeedback>
          <Text style={styles.past}>{past}</Text>
        </View>
        <View style={styles.nameContainer}>
          <Stars score={overall} />
          <Text style={styles.name}>{I18n.t('components.molecules.ListingReview.from', { name: decode(name) })}</Text>
        </View>
        <Text style={[styles.review, isEmpty(review) ? styles.noMessage : {}]}>
          {isEmpty(review) ? I18n.t('components.molecules.ListingReview.no_message') : decode(review)}
        </Text>
      </View>
    </View>
  );
});
