import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Alert, Keyboard } from 'react-native';
import { isEmpty } from 'lodash';

import ListingBasicInfo from '../components/templates/ListingBasicInfo';

import { createListing, updateListing, resetData } from '../reducers/createListing';
import { fetchListings } from '../reducers/storeListings';
import { getConfiguration } from '../reducers/config';

import {I18n} from '../langs/I18n';
class CreateListing extends PureComponent {
  componentDidMount() {
    this.props.getConfiguration();
  }

  handleSave = () => {
    const { productTitle, productType, price } = this.props;
    if (isEmpty(productTitle)) {
      Alert.alert('Listing title is required');
      return;
    }
    if (isEmpty(price)) {
      Alert.alert('Listing price is required');
      return;
    }
    if (isEmpty(productType)) {
      Alert.alert('Listing type is required');
      return;
    }
    Keyboard.dismiss();
    const {
      createListing, fetchListings, resetData, navigation,
    } = this.props;
    const screenKey = navigation.state.key;
    createListing((slug) => {
      fetchListings();
      resetData();
      Alert.alert('Listing created!', 'The listing has been created.', [
        {
          text: 'Back to Store',
          onPress: () => {
            navigation.pop();
          },
        },
        {
          text: 'See Listing',
          onPress: () => {
            navigation.navigate({
              routeName: 'Listing',
              params: { peerID: '', slug, screenKey },
            });
          },
        },
      ]);
    });
  };

  handleGoBack = () => {
    Alert.alert('Warning', 'If you go back, you will lose your progress', [
      { text: 'Cancel' },
      {
        text: 'OK',
        onPress: () => {
          this.props.resetData();
          this.props.navigation.goBack(null);
        },
      },
      { cancelable: false },
    ]);
  };

  handleGoToOptions = (option) => {
    this.props.navigation.navigate(option);
  };

  render() {
    return (
      <ListingBasicInfo
        editing
        toOptions={this.handleGoToOptions}
        onBack={this.handleGoBack}
        onSave={this.handleSave}
      />
    );
  }
}

const mapStateToProps = state => ({
  stage: state.createListing.stage,
  productTitle: state.createListing.title,
  price: state.createListing.price,
  productType: state.createListing.type,
});

const mapDispatchToProps = {
  resetData,
  createListing,
  updateListing,
  fetchListings,
  getConfiguration,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(CreateListing);
