import * as _ from 'lodash';
import { Dimensions } from 'react-native';

import { gatewayAPI, mbzGatewayAPI } from '../api/const';
import defaultImage from '../assets/images/defaultItem.png';
import defaultAvatar from '../assets/images/defaultAvatar.png';
import defaultHeader from '../assets/images/defaultHeader.png';
import selectorPlaceholder from '../assets/images/cameraPlaceholder.png';

import { serverConfig } from './server';

const DEVICE_WIDTH = Dimensions.get('window').width;
const DEVICE_HEIGHT = Dimensions.get('window').height;

export const getImageSource = hash => `${mbzGatewayAPI}/ob/images/${hash}`;

export const getLocalImageSource = hash => (hash === '' ? '' : `${gatewayAPI}/ob/images/${hash}`);

export const getImageSourceWithFallback = hash => ({
  first: { uri: getImageSource(hash) },
  second: {
    uri: getLocalImageSource(hash),
    headers: serverConfig.getAuthHeader(),
  },
  third: defaultImage,
});

export const getImageSourceWithDefault = hash =>
  (_.isEmpty(hash) ?
    defaultImage
    : {
      uri: getImageSource(hash),
    });

export const getImageSourceForSelector = hash =>
  (_.isEmpty(hash) ?
    selectorPlaceholder
    : {
      uri: getImageSource(hash),
    });

export const getAvatarImageSource = hash =>
  (_.isEmpty(hash) ?
    false
    : {
      uri: getImageSource(hash),
    });

export const getLocalAvatarImageSource = hash =>
  (_.isEmpty(hash) ?
    false
    : {
      uri: getLocalImageSource(hash),
      headers: serverConfig.getAuthHeader(),
    });

export const getHeaderImageSource = hash =>
  (_.isEmpty(hash) ?
    defaultHeader
    : {
      uri: getImageSource(hash),
    });

export const getLocalHeaderImageSource = hash =>
  (_.isEmpty(hash) ?
    defaultHeader
    : {
      uri: getLocalImageSource(hash),
      headers: serverConfig.getAuthHeader(),
    });
export const getImageSourceForImageViewer = (hash, fromLocal = false) =>
  (_.isEmpty(hash) ?
    defaultImage
    : {
      url: fromLocal ? getLocalImageSource(hash) : getImageSource(hash),
      width: DEVICE_WIDTH,
      height: DEVICE_HEIGHT,
      props: {
        source: { headers: fromLocal ? serverConfig.getAuthHeader() : {} },
        style: { resizeMode: 'contain' },
      },
    });
