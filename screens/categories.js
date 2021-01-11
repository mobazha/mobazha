import React, { PureComponent } from 'react';
import { View, FlatList, Platform } from 'react-native';
import { withNavigation } from 'react-navigation';

import Header from '../components/molecules/Header';
import { eventTracker } from '../utils/EventTracker';
import NavBackButton from '../components/atoms/NavBackButton';
import categories from '../config/categories';
import CatListItem from '../components/atoms/CatListItem';

import {I18n} from '../langs/I18n';

const renderingCategories = Platform.select({
  ios: categories.filter(cat => cat.name !== 'Gift Cards'),
  android: categories,
});

class Categories extends PureComponent {
  handleGoBack = () => {
    this.props.navigation.goBack();
  }

  handleGoToCategory = item => () => {
    const { navigation } = this.props;
    const onChangeCategory = navigation.getParam('onChangeCategory');

    if (onChangeCategory) {
      navigation.navigate('SubCategories', { item, onChangeCategory });
    } else {
      eventTracker.trackEvent('Discover-Tapped-Category', { category: item.name });
      navigation.navigate('CategoryOverview', { item });
    }
  };

  keyExtractor = (item, index) => `category_${index}`;

  renderItem = ({ item }) => (
    <CatListItem item={item} onPress={this.handleGoToCategory(item)} />
  );

  render() {
    return (
      <View style={{ flex: 1 }}>
        <Header left={<NavBackButton />} title="" onLeft={this.handleGoBack} />
        <FlatList
          style={{ flex: 1 }}
          data={renderingCategories}
          keyExtractor={this.keyExtractor}
          renderItem={this.renderItem}
        />
      </View>
    );
  }
}

export default withNavigation(Categories);
