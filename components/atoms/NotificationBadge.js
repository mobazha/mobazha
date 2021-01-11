import React from 'react';
import { View, Text } from 'react-native';

import { brandColor } from '../commonColors';
import { tabNotifBadgeSize } from '../../utils/navbar';

import {I18n} from '../../langs/I18n';

const styles = {
  notifCount: {
    ...tabNotifBadgeSize,
    borderRadius: 14,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: brandColor,
    elevation: 4,
    borderWidth: 2,
    borderColor: 'white',
  },
  notifCountText: {
    textAlign: 'center',
    fontSize: 12,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#ffffff',
  },
};

export default ({ notifCount, style, noBorder }) => {
  if (!notifCount) {
    return null;
  }

  return (
    <View style={[styles.notifCount, noBorder && { borderColor: brandColor }, style]}>
      <Text style={styles.notifCountText}>
        {`${notifCount}`}
      </Text>
    </View>
  );
};
