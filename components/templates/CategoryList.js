import React, { PureComponent } from 'react';
import { View, TouchableWithoutFeedback, Text, Dimensions, FlatList } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { withNavigation } from 'react-navigation';

import CatSliderItem from '../atoms/CatSliderItem';

import { homeCategories } from '../../config/categories';
import { secondaryTextColor, brandColor } from '../commonColors';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const { width: screenWidth } = Dimensions.get('screen');

const styles = {
  buttonWrapper: {
    width: '100%',
    paddingHorizontal: 15,
    paddingVertical: 5,
  },
  button: {
    width: '100%',
  },
  spacer: {
    height: 10,
  },
  slider: {
    width: '100%',
    height: 75,
    paddingTop: 7,
  },
  moreWrapper: {
    width: screenWidth / 5,
    height: 65,
    alignItems: 'center',
    justifyContent: 'center',
  },
  moreText: {
    fontSize: 12,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: secondaryTextColor,
  },
  moreIconWrapper: {
    width: 44,
    height: 44,
    marginBottom: 5,
    alignItems: 'center',
    justifyContent: 'center',
    paddingTop: 2,
    backgroundColor: 'white',
    borderRadius: 22,
  },
  wrapperStyle: {
    paddingTop: 16,
  },
};

class CategoryList extends PureComponent {
  handleGoToAllCategories = () => {
    eventTracker.trackEvent('Discover-Tapped-Category', { category: 'More' });
    this.props.navigation.navigate('Categories');
  };

  handleGoToCategoryOverview = item => () => {
    const { navigation } = this.props;
    eventTracker.trackEvent('Discover-Tapped-Category', { category: item.name });
    navigation.navigate('CategoryOverview', { item });
  };

  keyExtractor = (item, index) => `category_${index}`;

  renderMoreButton = () => (
    <TouchableWithoutFeedback onPress={this.handleGoToAllCategories}>
      <View style={styles.moreWrapper}>
        <View style={styles.moreIconWrapper}>
          <Ionicons name="ios-more" size={26} color={brandColor} />
        </View>
        <Text style={styles.moreText}>{I18n.t('components.templates.CategoryList.more')}</Text>
      </View>
    </TouchableWithoutFeedback>
  )

  renderSliderItem = ({ item }) => (item !== 'More Button' ? (
    <CatSliderItem
      slider
      item={item}
      onPress={this.handleGoToCategoryOverview(item)}
    />
  ) : this.renderMoreButton());

  render() {
    return (
      <View style={styles.wrapperStyle}>
        <FlatList
          data={homeCategories}
          keyExtractor={this.keyExtractor}
          renderItem={this.renderSliderItem}
          style={styles.slider}
          horizontal
        />
      </View>
    );
  }
}

export default withNavigation(CategoryList);
