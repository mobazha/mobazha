import React from 'react';
import { View, Dimensions } from 'react-native';
import ContentLoader, { Rect } from 'react-content-loader/native';
import { foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const IMAGE_RATIO = 328.0 / 750;

const IMAGE_WIDTH = SCREEN_WIDTH - 16;
const IMAGE_HEIGHT = SCREEN_WIDTH * IMAGE_RATIO - 16;

const styles = {
  wrapper: {
    margin: 8,
    width: IMAGE_WIDTH,
    height: IMAGE_HEIGHT,
  },
};

export default () => (
  <View style={styles.wrapper}>
    <ContentLoader
      width={IMAGE_WIDTH}
      height={IMAGE_HEIGHT}
      backgroundColor="#F3F3F3"
      foregroundColor={foregroundColor}
      viewBox={`0 0 ${IMAGE_WIDTH} ${IMAGE_HEIGHT}`}
    >
      <Rect
        x="0"
        y="0"
        rx="5"
        ry="5"
        width={IMAGE_WIDTH}
        height={IMAGE_HEIGHT}
      />
    </ContentLoader>
  </View>
);
