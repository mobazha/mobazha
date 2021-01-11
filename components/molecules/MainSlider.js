import React, { PureComponent } from 'react';
import { View, Dimensions, Text } from 'react-native';
import Carousel from 'react-native-snap-carousel';

import MainSliderItem from '../atoms/MainSliderItem';
import MainSliderContentLoader from '../atoms/MainSliderContentLoader';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const styles = {
  wrapper: {
    position: 'relative',
    marginTop: 8,
  },
  content: {
    position: 'relative',
  },
  hidden: {
    opacity: 0,
  },
  contentLoaderWrapper: {
    position: 'absolute',
    top: 0,
    left: 0,
  },
  dotStyle: {
    width: 12,
    height: 12,
    borderRadius: 6,
    backgroundColor: 'rgb(255, 255, 255)',
    borderStyle: 'solid',
    borderWidth: 0.3,
    borderColor: 'rgb(151, 151, 151)',
  },
  inactiveDotStyle: {
    width: 12,
    height: 12,
    borderRadius: 6,
    backgroundColor: 'rgb(255, 255, 255)',
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: 'rgb(151, 151, 151)',
  },
  pagination: {
    position: 'absolute',
    alignItems: 'center',
    justifyContent: 'center',
    top: SCREEN_WIDTH * 0.456 - 48,
    height: 24,
    right: 16,
    minWidth: 48,
    paddingHorizontal: 12,
    borderRadius: 12,
    backgroundColor: 'rgba(0, 0, 0, 0.4)',
  },
  paginationText: {
    color: 'white',
    fontSize: 12,
    fontWeight: 'bold',
  },
};

export default class MainSlider extends PureComponent {
  state = {
    activeSlide: 0,
    loadedItemCount: 0,
    loading: true,
  };

  handleLoadEnd = () => {
    const { loadedItemCount, loading } = this.state;
    const { items } = this.props;
    if (items.length > 0) {
      this.setState({
        loadedItemCount: (loadedItemCount + 1) % items.length,
        loading: loading && (loadedItemCount !== items.length - 1),
      });
    }
  }

  handleSnapToItem = (index) => {
    this.setState({ activeSlide: index });
  }

  renderItem = ({ item, index }) => (
    <MainSliderItem source={item} key={`main_slider_${index}`} onLoadEnd={this.handleLoadEnd} />
  );

  render() {
    const { items = [] } = this.props;
    const { loading, activeSlide } = this.state;
    return (
      <View style={styles.wrapper}>
        <View style={[styles.content, loading && styles.hidden]}>
          <Carousel
            data={items}
            renderItem={this.renderItem}
            sliderWidth={SCREEN_WIDTH}
            itemWidth={SCREEN_WIDTH}
            itemHeight={SCREEN_WIDTH * 0.544}
            sliderHeight={SCREEN_WIDTH}
            onSnapToItem={this.handleSnapToItem}
            enableSnap
            enableMomentum
            loop
            autoplay
            autoplayDelay={1000}
            autoplayInterval={4000}
            inactiveSlideOpacity={1}
          />
          <View style={styles.pagination}>
            <Text style={styles.paginationText}>{activeSlide + 1}/{items.length}</Text>
          </View>
        </View>
        {loading && (
          <View style={styles.contentLoaderWrapper}>
            <MainSliderContentLoader />
          </View>
        )}
      </View>
    );
  }
}
