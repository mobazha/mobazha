import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';

import LinkText from '../components/atoms/LinkText';
import NavBackButton from '../components/atoms/NavBackButton';
import Header from '../components/molecules/Header';
import InventoryEditor from '../components/templates/InventoryEditor';

import { screenWrapper } from '../utils/styles';

import { updateInventory } from '../reducers/createListing';

import {I18n} from '../langs/I18n';
class EditInventory extends PureComponent {
  constructor(props) {
    super(props);
    const { inventory } = props;
    const idx = props.navigation.getParam('idx');
    if (idx >= 0) {
      const {
        productID, surcharge, quantity, variantCombo,
      } = inventory[idx];
      this.state = {
        productID,
        surcharge,
        quantity,
        variantCombo,
      };
    } else {
      this.state = {
        productID: '',
        surcharge: '0',
        quantity: '-1',
        variantCombo: [],
      };
    }
  }

  render() {
    const { options, price } = this.props;
    const { productID, surcharge, quantity, variantCombo } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.goBack();
          }}
          title={I18n.t('screens.editInventory.edit_variant_combo')}
          right={<LinkText text={I18n.t('screens.editInventory.apply')} />}
          onRight={() => {
            const idx = this.props.navigation.getParam('idx');
            this.props.updateInventory({
              idx,
              value: this.state,
            });
            this.props.navigation.goBack();
          }}
        />
        <InventoryEditor
          inventory={{ productID, surcharge, quantity, variantCombo }}
          price={price}
          options={options}
          onChange={inventory => this.setState({ ...inventory })}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  price: state.createListing.price,
  inventory: state.createListing.inventory,
  options: state.createListing.options,
});

const mapDispatchToProps = {
  updateInventory,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(EditInventory);
