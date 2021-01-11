import React from 'react';
import { Dimensions, View } from 'react-native';
import ContentLoader, { Rect } from 'react-content-loader/native';

import { foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const CARD_WIDTH = (SCREEN_WIDTH - 30) / 2;
const CARD_HEIGHT = CARD_WIDTH / 2 + 8;
const WIDTH = SCREEN_WIDTH - 8;
const HEIGHT = (CARD_HEIGHT * 2) + 2;

const styles = {
  wrapper: {
    paddingHorizontal: 14,
    marginTop: 16,
  },
};

export default () => (
  <View style={styles.wrapper}>
    <ContentLoader
      width={WIDTH}
      height={HEIGHT}
      backgroundColor="#F3F3F3"
      foregroundColor={foregroundColor}
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
    >
      <Rect
        x="0"
        y="0"
        rx="2"
        ry="2"
        width={CARD_WIDTH}
        height={CARD_HEIGHT}
      />

      <Rect
        x={CARD_WIDTH + 2}
        y="0"
        rx="2"
        ry="2"
        width={CARD_WIDTH}
        height={CARD_HEIGHT}
      />

      <Rect
        x="0"
        y={CARD_HEIGHT + 2}
        rx="2"
        ry="2"
        width={CARD_WIDTH}
        height={CARD_HEIGHT}
      />

      <Rect
        x={CARD_WIDTH + 2}
        y={CARD_HEIGHT + 2}
        rx="2"
        ry="2"
        width={CARD_WIDTH}
        height={CARD_HEIGHT}
      />
    </ContentLoader>
  </View>
);
