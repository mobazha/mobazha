import React from 'react';
import { View, Text } from 'react-native';
import { isEmpty, get } from 'lodash';
import decode from 'unescape';
import { withNavigation } from 'react-navigation';

import { staticLabelColor } from '../commonColors';
import AvatarImage from '../atoms/AvatarImage';
import Stars from '../atoms/Stars';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    paddingBottom: 14,
  },
  topWrapper: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  profileImg: {
    width: 38,
    height: 38,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 10,
  },
  content: {
    flex: 1,
    flexDirection: 'column',
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
    marginBottom: 4,
  },
  review: {
    width: '100%',
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    color: 'black',
  },
  reviewInDetail: {
    fontSize: 15,
  },
  noMessage: {
    color: staticLabelColor,
    fontStyle: 'italic',
  },
};

export default withNavigation(({
  profile, review, overall, style, past, navigation, inDetail,
}) => {
  const name = get(profile, 'name', 'Anonymous');
  const small = get(profile, 'avatarHashes.small');
  const peerID = get(profile, 'peerID');
  const isNavigatable = inDetail && name !== 'Anonymous';
  const navigateToStore = () => {
    if (isNavigatable) {
      navigation.push('ExternalStore', { peerID });
    }
  };
  return (
    <View style={[styles.wrapper, style]}>
      <AvatarImage
        style={styles.profileImg}
        thumbnail={small}
        onPress={navigateToStore}
      />
      <View style={styles.content}>
        {overall && (
          <View style={styles.topWrapper}>
            <Stars score={overall} />
            <Text style={styles.past}>{past}</Text>
          </View>
        )}
        <Text style={styles.name}>{I18n.t('components.molecules.BuyerReview.from', {name:decode(name)})}</Text>
        <Text style={[
          styles.review,
          isEmpty(review) ? styles.noMessage : {},
          inDetail ? styles.reviewInDetail : {},
        ]}
        >
          {isEmpty(review) ? I18n.t('components.molecules.BuyerReview.no_review') : decode(review)}
        </Text>
      </View>
    </View>
  );
});
