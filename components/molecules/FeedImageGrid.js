import React from 'react';
import { Dimensions, FlatList, TouchableWithoutFeedback, View, Text } from 'react-native';
import FastImage from 'react-native-fast-image';
import { get } from 'lodash';

import OBFastImage from '../atoms/OBFastImage';

import { getImageSourceWithDefault, getImageSourceForImageViewer } from '../../utils/files';
import { chatStyles } from '../../utils/styles';
import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const DEVICE_WIDTH = Dimensions.get('window').width;
const RIGHT_PADDING = chatStyles.avatarImage.marginLeft;
const LEFT_PADDING = chatStyles.avatarImage.width + chatStyles.avatarImage.marginLeft * 2;
const getImageItemWidth = count =>
  (DEVICE_WIDTH - RIGHT_PADDING - LEFT_PADDING - (count - 1) * RIGHT_PADDING / 2) / count;

const getImageItemMarginBottom = isFirstOfGrid => (isFirstOfGrid ? RIGHT_PADDING / 2 : 5);

const styles = {
  imagesListContainer: (count, isFirstOfGrid) => ({
    height: getImageItemWidth(count) + 10 + getImageItemMarginBottom(isFirstOfGrid),
    paddingTop: 10,
  }),
  imagesList: (count, isFirstOfGrid) => ({
    height: getImageItemWidth(count),
    marginBottom: getImageItemMarginBottom(isFirstOfGrid),
  }),
  imageItem: (count, index) => ({
    width: getImageItemWidth(count),
    height: '100%',
    marginLeft: index === 0 ? RIGHT_PADDING : RIGHT_PADDING / 2,
  }),
  itemContainer: {
    alignSelf: 'baseline',
    flex: 1,
  },
  image: {
    height: '100%',
    width: '100%',
  },
  moreOverlay: {
    position: 'absolute',
    height: '100%',
    width: '100%',
    backgroundColor: 'white',
    opacity: 0.7,
  },
  textContainer: {
    position: 'absolute',
    width: '100%',
    height: '100%',
    justifyContent: 'center',
    alignItems: 'center',
  },
  moreOverlayText: {
    fontSize: 28,
    color: primaryTextColor,
  },
};

class FeedImageGrid extends React.PureComponent {
  renderImageItem = (images, moreCount) => ({ item: image, index }) => {
    const { onViewImages } = this.props;
    const { medium } = image;
    const showMoreOverlay = moreCount && index === get(images, 'length', 0) - 1;
    return (
      <TouchableWithoutFeedback
        onPress={() =>
          onViewImages(this.props.images.map(image => getImageSourceForImageViewer(image.medium)))
        }
      >
        <View style={styles.itemContainer}>
          <View style={styles.imageItem(get(images, 'length', 0), index)}>
            <OBFastImage
              style={styles.image}
              hash={medium}
              resizeMode={FastImage.resizeMode.cover}
            />
            {showMoreOverlay && <View style={styles.moreOverlay} />}
            {showMoreOverlay && (
              <View style={styles.textContainer}>
                <Text style={styles.moreOverlayText}>{`+${moreCount}`}</Text>
              </View>
            )}
          </View>
        </View>
      </TouchableWithoutFeedback>
    );
  };

  renderFlatList(images, isFirstOfGrid, moreCount) {
    return (
      <FlatList
        style={styles.imagesList(get(images, 'length', 0), isFirstOfGrid)}
        keyExtractor={(item, index) => `image_item_${index}`}
        data={images}
        renderItem={this.renderImageItem(images, moreCount)}
        horizontal
        pointerEvents="auto"
        scrollEnabled={false}
      />
    );
  }

  render() {
    const images = this.props.images || [];
    const count = images.length;

    if (count === 0) {
      return null;
    } else if (count < 4) {
      return <View style={styles.imagesListContainer(count, false)}>{this.renderFlatList(images, false)}</View>;
    } else {
      const imagesRow1 = images.slice(0, 2);
      const imagesRow2 = images.slice(2, Math.min(count, 5));

      return (
        <View
          style={{
            height:
              styles.imagesListContainer(imagesRow1.length, true).height +
              styles.imagesListContainer(imagesRow2.length, false).height - 10,
            paddingTop: 10,
          }}
        >
          {this.renderFlatList(imagesRow1, true)}
          {this.renderFlatList(imagesRow2, false, count > 5 ? count - 5 : null)}
        </View>
      );
    }
  }
}

export default FeedImageGrid;
