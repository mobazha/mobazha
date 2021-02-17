import React, { PureComponent } from 'react';
import { View } from 'react-native';

import Header from '../components/molecules/Header';
import NavBackButton from '../components/atoms/NavBackButton';
import PurchaseStateTemplate from '../components/templates/PurchaseState';

import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';
export default class ExternalPaySuccess extends PureComponent {
  handleGoBack = () => {
    const { navigation } = this.props;
    const screenKey = navigation.getParam('screenKey');
    navigation.goBack(screenKey);
  }

  generatePaidOrderInfo = () => {
    const { navigation } = this.props;
    const orderId = navigation.getParam('orderId');
    return { orderId };
  }

  render() {
    const { navigation } = this.props;
    return (
      <View style={screenWrapper.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.handleGoBack} />
        <PurchaseStateTemplate
          paymentState="success"
          paidOrderInfo={this.generatePaidOrderInfo()}
          navigation={navigation}
        />
      </View>
    );
  }
}
