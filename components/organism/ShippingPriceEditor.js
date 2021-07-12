import React from 'react';
import { connect } from 'react-redux';

import TextInput from '../atoms/TextInput';
import InputGroup from '../atoms/InputGroup';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';
class ShippingPriceEditor extends React.Component {
  onChangeName = (name) => {
    const { item, pos, onChange } = this.props;
    onChange({ ...item, name }, pos);
  };

  onChangePrice = (formated, price) => {
    const { item, pos, onChange } = this.props;
    onChange({ ...item, price }, pos);
  }

  onChangeEstimate = (estimatedDelivery) => {
    const { item, pos, onChange } = this.props;
    onChange({ ...item, estimatedDelivery }, pos);
  }

  onChangeAdditionalPrice = (formated, additionalItemPrice) => {
    const { item, pos, onChange } = this.props;
    onChange({ ...item, additionalItemPrice }, pos);
  }

  render() {
    const { item, pos, localSymbol, removeItem, hideDelete, localMask } = this.props;
    const { name, price, estimatedDelivery, additionalItemPrice } = item;
    return (
      <InputGroup
        title={I18n.t('components.organism.ShippingPriceEditor.shipping_service', {pos: pos + 1})}
        action={!hideDelete && removeItem}
        actionTitle={!hideDelete && I18n.t('components.organism.ShippingPriceEditor.delete')}
      >
        <TextInput
          title={I18n.t('components.organism.ShippingPriceEditor.service')}
          value={name}
          required
          placeholder={I18n.t('components.organism.ShippingPriceEditor.shipping_hint')}
          onChangeText={this.onChangeName}
        />
        <TextInput
          title={I18n.t('components.organism.ShippingPriceEditor.Duration')}
          defaultValue={estimatedDelivery}
          placeholder={I18n.t('components.organism.ShippingPriceEditor.Duration_hint')}
          required
          onChangeText={this.onChangeEstimate}
        />
        <TextInput
          title={I18n.t('components.organism.ShippingPriceEditor.price')}
          value={price}
          required
          placeholder={`${localSymbol}0.00`}
          mask={localMask}
          onChangeText={this.onChangePrice}
          keyboardType="decimal-pad"
        />
        <TextInput
          title={I18n.t('components.organism.ShippingPriceEditor.additional_price')}
          noBorder
          value={additionalItemPrice}
          placeholder={`${localSymbol}0.00`}
          mask={localMask}
          onChangeText={this.onChangeAdditionalPrice}
          keyboardType="decimal-pad"
        />
      </InputGroup>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(ShippingPriceEditor);
