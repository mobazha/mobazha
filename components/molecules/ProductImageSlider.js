import React, { PureComponent } from 'react';
import { View, Image, Dimensions, TouchableWithoutFeedback, Text } from 'react-native';
import Carousel from 'react-native-snap-carousel';

import { getImageSourceWithDefault } from '../../utils/files';

import {I18n} from '../../langs/I18n';

const { width } = Dimensions.get('window');

const styles = {
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(108, 108, 108, 0.5)',
    borderRadius: 16,
    height: 32,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    bottom: 16,
    right: 16,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
};

export default class ProductImageSlider extends PureComponent {
  state = {
    activeSlide: 0,
  };

  handleSnapToItem = (index) => {
    this.setState({ activeSlide: index });
  }

  renderItem = ({ item, index }) => {
    const { activeSlide } = this.state;
    return (
      <TouchableWithoutFeedback
        activeOpacity={1}
        key={index}
        onPress={() => this.props.onShowImage(activeSlide)}
      >
        <View style={{ width }}>
          <Image
            style={{
              width,
              height: width,
            }}
            source={getImageSourceWithDefault(item.medium)}
            resizeMode="cover"
          />
        </View>
      </TouchableWithoutFeedback>
    );
  };

  render() {
    const { images } = this.props;
    const { activeSlide } = this.state;
    return (
      <View>
        <Carousel
          data={images}
          renderItem={this.renderItem}
          hasParallaxImages
          sliderWidth={width}
          itemWidth={width}
          itemHeight={width}
          sliderHeight={width}
          onSnapToItem={this.handleSnapToItem}
          enableSnap
          loop
          autoplay={false}
          autoplayDelay={5000}
          autoplayInterval={5000}
          inactiveSlideOpacity={1}
        />
        <View style={styles.overlay}>
          <Text style={styles.overlayText}>{activeSlide + 1}/{images.length}</Text>
        </View>
      </View>
    );
  }
}
