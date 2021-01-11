import React, { useState, useEffect } from 'react';
import { Image, TouchableWithoutFeedback, View } from 'react-native';
import FastImage from 'react-native-fast-image';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { foregroundColor } from '../commonColors';

import { getAvatarImageSource, getLocalAvatarImageSource } from '../../utils/files';

import defaultAvatar from '../../assets/images/defaultAvatar.png';

import {I18n} from '../../langs/I18n';

const styles = {
  imageContainer: width => ({
    width,
    height: width,
    borderRadius: width / 2,
    borderWidth: 2,
    borderColor: 'white',
    borderStyle: 'solid',
    shadowColor: 'rgba(0, 0, 0, 0.5)',
    shadowOffset: {
      width: 0,
      height: 1,
    },
    shadowRadius: 2,
    shadowOpacity: 1,
    elevation: 3,
    backgroundColor: '#FFF',
  }),
  image: width => ({
    position: 'absolute',
    width: '100%',
    height: '100%',
    borderRadius: (width - 4) * 0.5,
  }),
  cameraContainer: {
    width: '100%',
    height: '100%',
    backgroundColor: '#5d5d5d8c',
    alignItems: 'center',
    justifyContent: 'center',
  },
};

export default ({
  thumbnail, onPress, style, showCamera, showLocal,
}) => {
  const [failed, setFailed] = useState(false);
  useEffect(() => { setFailed(false); }, [thumbnail]);
  const source = showLocal ? getLocalAvatarImageSource(thumbnail) : getAvatarImageSource(thumbnail);
  return (
    <TouchableWithoutFeedback disabled={!onPress} onPress={onPress} >
      <View style={[styles.imageContainer(style.width), style]}>
        <FastImage
          style={styles.image(style.width)}
          source={(failed || !source) ? defaultAvatar : source}
          resizeMode="cover"
          onError={() => setFailed(true)}
          fallback
        />
        {showCamera && (
          <View style={[styles.image(style.width), styles.cameraContainer]}>
            <Ionicons name="md-camera" size={24} color={foregroundColor} />
          </View>
        )}
      </View>
    </TouchableWithoutFeedback>
  );
};
