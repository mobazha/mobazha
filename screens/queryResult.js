import React, { PureComponent } from 'react';
import { Text, View } from 'react-native';

import Header from '../components/molecules/Header';
import InfiniteProducts from '../components/templates/InfiniteProducts';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';
class QueryResult extends PureComponent {
  goBack = () => {
    const { navigation } = this.props;
    navigation.goBack();
  };

  handleGoToListingDetails = (params) => {
    this.props.navigation.navigate('Listing', params);
  }

  render() {
    const queryString = this.props.navigation.getParam('query');
    return (
      <View style={screenWrapper.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.goBack} title="商品" />
        <InfiniteProducts
          queryString={queryString}
          toListingDetails={this.handleGoToListingDetails}
          showInitialLoading
        />
      </View>
    );
  }
}

export default QueryResult;
