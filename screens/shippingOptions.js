import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';

import Header from '../components/molecules/Header';
import ListShippingMethod from '../components/templates/ListShippingMethod';
import NavBackButton from '../components/atoms/NavBackButton';

import { removeShippingOption } from '../reducers/createListing';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';
class ShippingOptions extends PureComponent {
  onAdd = () => {
    this.props.navigation.navigate('AddShippingMethod');
  };

  onRemove = (idx) => {
    this.props.removeShippingOption(idx);
  };

  onEdit = (idx) => {
    this.props.navigation.navigate('AddShippingMethod', { idx });
  };

  render() {
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          title="Shipping"
          noBorder
        />
        <ListShippingMethod onAdd={this.onAdd} onRemove={this.onRemove} onEdit={this.onEdit} />
      </View>
    );
  }
}

const mapDispatchToProps = {
  removeShippingOption,
};

export default connect(
  null,
  mapDispatchToProps,
)(ShippingOptions);
