import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';

import Header from '../components/molecules/Header';
import ListingCustomOptions from '../components/templates/ListingCustomOptions';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';
import LinkText from '../components/atoms/LinkText';

import { updateInventory } from '../reducers/createListing';

import {I18n} from '../langs/I18n';
class CustomOptions extends PureComponent {
  onUpdateInventory = (info) => {
    this.props.updateInventory({ idx: 0, value: info });
  }

  goBack = () => {
    this.props.navigation.goBack();
  };

  handleAddNew = () => {
    this.props.navigation.navigate('EditVariants');
  };

  handleEdit = () => {
    this.props.navigation.navigate('EditVariants', { editing: true });
  }

  toInventoryItem = (idx) => {
    this.props.navigation.navigate({
      routeName: 'EditInventory',
      params: {
        idx,
      },
    });
  };

  saveInventory = () => {
    this.props.navigation.goBack();
  };

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.goBack}
          title={I18n.t('screens.customOptions.Variants_Inventory')}
          right={<LinkText text={I18n.t('screens.customOptions.Save')} />}
          onRight={this.saveInventory}
        />
        <ListingCustomOptions
          onAddNew={this.handleAddNew}
          onEdit={this.handleEdit}
          toInventoryItem={this.toInventoryItem}
          onUpdateInventory={this.onUpdateInventory}
        />
      </View>
    );
  }
}

const mapStateToProps = ({ createListing: { inventoryTracking, options } }) => ({
  inventoryTracking,
  options,
});
const mapDispatchToProps = {
  updateInventory,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(CustomOptions);
