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
      Alert.alert(I18n.t('screens.createListing.title_required'));
      return;
    }
    if (isEmpty(price)) {
      Alert.alert(I18n.t('screens.createListing.price_required'));
      return;
    }
    if (isEmpty(productType)) {
      Alert.alert(I18n.t('screens.createListing.type_required'));
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
      Alert.alert(I18n.t('screens.createListing.listing_created'), I18n.t('screens.createListing.has_created'), [
        {
          text: I18n.t('screens.createListing.back_to_store'),
          onPress: () => {
            navigation.pop();
          },
        },
        {
          text: I18n.t('screens.createListing.see_listing'),
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
    Alert.alert(I18n.t('screens.createListing.warning'), I18n.t('screens.createListing.warning_info'), [
      { text: I18n.t('screens.createListing.cancel') },
      {
        text: I18n.t('screens.createListing.ok'),
        onPress: () => {
          this.props.resetData();
          this.props.navigation.goBack(null);
        },
      }
    ], { cancelable: false });
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
