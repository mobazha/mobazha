import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Alert } from 'react-native';
import { isEmpty } from 'lodash';

import ListingBasicInfo from '../components/templates/ListingBasicInfo';

import { updateListing, resetData } from '../reducers/createListing';
import { getConfiguration } from '../reducers/config';

import {I18n} from '../langs/I18n';
class EditListing extends PureComponent {
  componentDidMount() {
    this.props.getConfiguration();
  }

  handleGoBack = () => {
    this.props.resetData();
    this.props.navigation.goBack(null);
  }

  handleSave = () => {
    const {
      productTitle, productType, price, slug,
    } = this.props;
    if (isEmpty(productTitle)) {
      Alert.alert('Listing title is required');
      return false;
    }
    if (parseFloat(price) <= 0) {
      Alert.alert('Listing price is required');
      return false;
    }
    if (isEmpty(productType)) {
      Alert.alert('Listing type is required');
      return false;
    }
    this.props.updateListing({ slug, peerID: '' });
    this.props.navigation.goBack();
  }


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
  productTitle: state.createListing.title,
  price: state.createListing.price,
  productType: state.createListing.type,
  slug: state.createListing.slug,
});

const mapDispatchToProps = {
  resetData,
  updateListing,
  getConfiguration,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(EditListing);
