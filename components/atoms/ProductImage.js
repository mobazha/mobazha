import React from 'react';
import { View } from 'react-native';
import FastImage from 'react-native-fast-image';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { get, isEmpty } from 'lodash';

import { getImageSourceForSelector } from '../../utils/files';
import { borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  image: {
    width: 110,
    height: 110,
    borderRightWidth: 1,
    borderRightColor: borderColor,
    alignItems: 'center',
    justifyContent: 'center',
  },
};

export default ({ images }) => {
  const thumbnail = get(images, '[0].hashes.small');
  return isEmpty(thumbnail) ?
    (
      <View style={styles.image}>
        <Ionicons name="md-camera" size={25} color="#c8c7cc" />
      </View>
    ) : (
      <FastImage
        style={styles.image}
        source={getImageSourceForSelector(thumbnail)}
        resizeMode={FastImage.resizeMode.cover}
      />
    );
};
