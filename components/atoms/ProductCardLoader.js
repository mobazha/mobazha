import React from 'react';
import { Dimensions, View } from 'react-native';
import ContentLoader, { Rect } from 'react-content-loader/native';


import { foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const COMPACT_WIDTH = (SCREEN_WIDTH - 32) * 0.31;
const COMPACT_HEIGHT = SCREEN_WIDTH * 0.42;

const COMAPCT_MARGIN = SCREEN_WIDTH * 0.03;

const WIDTH = SCREEN_WIDTH * 0.49;
const HEIGHT = SCREEN_WIDTH * 0.65;
const MARGIN = SCREEN_WIDTH * 0.02;

const styles = {
  wrapper: {
    width: WIDTH,
    height: HEIGHT,
    backgroundColor: foregroundColor,
    marginHorizontal: 5,
  },

  compactWrapper: {
    width: COMPACT_WIDTH,
    height: COMPACT_HEIGHT,
    borderRadius: 2,
    backgroundColor: foregroundColor,
  },
};

const renderCardPlaceholder = (x, y, width, height, compact) => (
  <React.Fragment>
    <Rect
      x={x}
      y={y}
      rx="2"
      ry="2"
      width={`${width}`}
      height={`${compact ? height - 45 : height - 60}`}
    />
    <Rect
      x={x}
      y={`${compact ? height - 40 : height - 55}`}
      rx="2"
      ry="2"
      width={`${width * 0.8}`}
      height="10"
    />
    {!compact && (
      <Rect
        x={x}
        y={`${height - 40}`}
        rx="2"
        ry="2"
        width={`${width * 0.6}`}
        height="10"
      />
    )}
  </React.Fragment>
);

const ProductCardLoader = ({ compact }) => {
  const width = compact ? COMPACT_WIDTH : WIDTH;
  const height = compact ? COMPACT_HEIGHT : HEIGHT;
  const margin = compact ? COMAPCT_MARGIN : MARGIN;
  const FULL_WIDTH = compact ? SCREEN_WIDTH - 32 : SCREEN_WIDTH - 8;
  return (
    <View style={compact ? styles.compactWrapper : styles.wrapper}>
      <ContentLoader
        width={FULL_WIDTH}
        height={height}
        backgroundColor="#F3F3F3"
        foregroundColor={foregroundColor}
        viewBox={`0 0 ${FULL_WIDTH} ${height}`}
      >
        {renderCardPlaceholder(0, 0, width, height, compact)}
        {renderCardPlaceholder(`${width + margin}`, 0, width, height, compact)}
        {compact && renderCardPlaceholder(`${(width + margin) * 2}`, 0, width, height, compact)}
      </ContentLoader>
    </View>
  );
};

export default ProductCardLoader;
