import React from 'react';
import { View, Image, TouchableWithoutFeedback, Platform, ActivityIndicator } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { isEmpty, hasIn } from 'lodash';

import { chatStyles } from '../../utils/styles';
import { getImageSourceForImageViewer } from '../../utils/files';
import { serverConfig } from '../../utils/server';

import {I18n} from '../../langs/I18n';

const MAJOR_PADDING = chatStyles.avatarImage.marginLeft;

const styles = {
  wrapper: {
    backgroundColor: 'transparent',
    padding: MAJOR_PADDING * 2 / 3,
  },
  button: {
    width: '100%',
    height: '100%',
    justifyContent: 'center',
    alignItems: 'center',
    borderWidth: 1,
    borderColor: '#c8c7cc',
  },
  image: {
    width: '100%',
    height: '100%',
  },
  close: {
    position: 'absolute',
    justifyContent: 'center',
    alignItems: 'center',
    right: 0,
    width: 30,
    height: 30,
    borderRadius: 15,
    paddingTop: Platform.OS === 'ios' ? 5 : 0,
    paddingLeft: Platform.OS === 'ios' ? 2 : 0,
    backgroundColor: '#6c6c6c7F',
  },
  removeButton: {
    width: 16,
    height: 16,
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
    position: 'absolute',
    right: -10,
    top: -10,
    backgroundColor: '#FFFFFF',
  },
  star: {
    position: 'absolute',
    left: 0,
  },
};

export default ({
  style, img, onPress, onRemove, isDefault, loading,
}) => {
  if (loading) {
    return (
      <View style={[styles.wrapper, style]}>
        <View style={styles.button}>
          <ActivityIndicator size="large" />
        </View>
      </View>
    );
  }
  return (isEmpty(img) ? (
    <View style={[styles.wrapper, style]}>
      <TouchableWithoutFeedback onPress={onPress}>
        <View style={styles.button}>
          <Ionicons name="md-camera" size={25} color="#c8c7cc" />
        </View>
      </TouchableWithoutFeedback>
    </View>
  ) : (
    <View style={[styles.wrapper, style]}>
      <TouchableWithoutFeedback onPress={onPress}>
        <View style={styles.button}>
          <Image
            style={styles.image}
            source={ hasIn(img, 'hashes') ?
              getImageSourceForImageViewer(img)
            : {
              uri: img,
              headers: serverConfig.getAuthHeader(),
            }}
            resizeMode="cover"
          />
        </View>
      </TouchableWithoutFeedback>
      <TouchableWithoutFeedback onPress={onRemove}>
        <View style={styles.close}>
          <Ionicons name="md-close" size={20} color="#FFF" />
        </View>
      </TouchableWithoutFeedback>
      {
        isDefault && <Ionicons
          style={styles.star}
          name={Platform.OS === 'ios' ? 'ios-star' : 'md-star'}
          size={24}
          color="#00bf65"
        />
      }
    </View>
  ));
};
