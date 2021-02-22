import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { get, isEmpty } from 'lodash';
import decode from 'unescape';

import AvatarImage from '../atoms/AvatarImage';
import LocationPin from '../atoms/LocationPin';
import { starRatingColor, primaryTextColor, secondaryTextColor, borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'flex-start',
    paddingVertical: 14,
    paddingHorizontal: 15,
    borderBottomWidth: 1,
    borderColor,
  },
  avatarImage: {
    width: 40,
    height: 40,
  },
  content: {
    flex: 1,
    marginLeft: 8,
    flexDirection: 'column',
  },
  name: {
    fontSize: 15,
    fontWeight: 'bold',
    color: primaryTextColor,
    lineHeight: 17,
    marginBottom: 4,
  },
  descriptionWrapper: {
    flex: 1,
    flexDirection: 'row',
  },
  description: {
    flex: 1,
    fontSize: 15,
    lineHeight: 18,
    marginBottom: 10,
    color: primaryTextColor,
  },
  infos: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  location: {
    marginRight: 15,
    paddingVertical: 0,
  },
  rating: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
};

export default ({ profile, onPress }) => {
  const avatarHash = get(profile, 'data.profile.avatarHashes.small');
  const name = decode(get(profile, 'data.profile.name'));
  const shortDescription = decode(get(profile, 'data.profile.shortDescription'));
  const location = decode(get(profile, 'data.profile.location')) || '';
  const averageRating = get(profile, 'data.profile.stats.averageRating', 0);
  const ratingCount = get(profile, 'data.profile.stats.ratingCount', 0);
  const peerID = get(profile, 'data.profile.peerID');
  return (
    <TouchableWithoutFeedback onPress={() => onPress(peerID)}>
      <View style={styles.wrapper}>
        <AvatarImage thumbnail={avatarHash} style={styles.avatarImage} />
        <View style={styles.content}>
          <Text style={styles.name}>{decode(name)}</Text>
          <View style={styles.descriptionWrapper}>
            {!isEmpty(decode(shortDescription)) && (
              <Text style={styles.description} numberOfLines={3}>{decode(shortDescription)}</Text>
            )}
          </View>
          <View style={styles.infos}>
            <LocationPin secondary location={decode(location)} style={styles.location} />
            <Text style={styles.rating}>
              &nbsp;
              <Ionicons name="md-star" size={16} color={starRatingColor} />
              &nbsp;
              {`${averageRating.toFixed(1)} (${ratingCount})`}
            </Text>
          </View>
        </View>
      </View>
    </TouchableWithoutFeedback>
  );
};
