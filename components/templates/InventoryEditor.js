import React, { PureComponent } from 'react';
import { View, Text, FlatList } from 'react-native';
import { connect } from 'react-redux';

import InputGroup from '../atoms/InputGroup';
import TextInput from '../atoms/TextInput';
import CheckBox from '../atoms/CheckBox';

import { borderColor, formLabelColor, primaryTextColor, greenColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  quantityEditor: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  quantityPlaceholder: {
    flexDirection: 'row',
    justifyContent: 'flex-start',
    paddingVertical: 20,
    flex: 1,
  },
  textEditPart: {
    flex: 1,
  },
  label: {
    width: 150,
    fontSize: 15,
    color: '#8d8d8d',
    textAlign: 'left',
  },
  variant: {
    borderWidth: 1,
    borderColor: '#c8c7cc',
    paddingVertical: 7,
    paddingHorizontal: 11,
    fontSize: 13,
    fontWeight: 'bold',
    color: primaryTextColor,
  },
  variantHolder: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 21,
    borderBottomWidth: 1,
    borderBottomColor: borderColor,
  },
  variantTitle: {
    width: 150,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
  },
  description: {
    paddingHorizontal: 16,
    fontSize: 13,
    color: '#8d8d8d',
    textAlign: 'left',
    marginTop: 12,
  },
  surchargeHolder: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    borderBottomWidth: 1,
    borderColor: '#e8e8e8',
  },
  surchargeInput: {
    flex: 1,
  },
  surChargePrice: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  totalPriceLabel: {
    color: greenColor,
    fontSize: 15,
  },
  totalPrice: {
    color: greenColor,
    fontSize: 15,
    fontWeight: 'bold',
  },
};

const getVariantList = (options, variantCombo) =>
  variantCombo.map((variant, idx) => ({
    name: options[idx].name,
    value: options[idx].variants[variant],
  }));

class InventoryEditor extends PureComponent {
  onChangeQuantity = (quantity) => {
    this.props.onChange({
      ...this.props.inventory,
      quantity,
    });
  };

  onChangeUnlimited = () => {
    const {
      inventory: { quantity },
    } = this.props;
    this.props.onChange({
      ...this.props.inventory,
      quantity: parseInt(quantity, 10) === -1 ? '0' : '-1',
    });
  };


  onChangeSurcharge = (formated, surcharge) => {
    this.props.onChange({
      ...this.props.inventory,
      surcharge,
    });
  };

  onChangeProductId = (productID) => {
    this.props.onChange({
      ...this.props.inventory,
      productID,
    });
  };

  variantKeyExtractor = ({ index }) => `variant_${index}`;

  renderVariant = ({ item }) => (
    <View style={styles.variantHolder}>
      <Text style={styles.variantTitle}>{item.name}</Text>
      <Text style={styles.variant}>{item.value}</Text>
    </View>
  );

  render() {
    const { inventory, price, options, localSymbol, localLabelFromLocal, localMask } = this.props;
    const { productID, surcharge, quantity, variantCombo } = inventory;
    const variants = getVariantList(options, variantCombo);
    const totalPrice = parseFloat(price) + parseFloat(surcharge || '0');
    return (
      <React.Fragment>
        <InputGroup title={I18n.t('components.templates.InventoryEditor.details')}>
          <FlatList
            data={variants}
            keyExtractor={this.variantKeyExtractor}
            renderItem={this.renderVariant}
          />
          <View style={styles.surchargeHolder}>
            <View style={styles.surchargeInput}>
              <TextInput 
                title={I18n.t('components.templates.InventoryEditor.surcharge')}
                noBorder
                placeholder={`${localSymbol}0.00`}
                value={surcharge || ''}
                mask={localMask}
                onChangeText={this.onChangeSurcharge}
                keyboardType="decimal-pad"
              />
            </View>
            <View style={styles.surChargePrice}>
              <Text style={styles.totalPriceLabel}>{I18n.t('components.templates.InventoryEditor.total')} </Text>
              <Text style={styles.totalPrice}>{localLabelFromLocal(totalPrice)}</Text>
            </View>
          </View>
          <TextInput
            title={I18n.t('components.templates.InventoryEditor.sku')}
            placeholder={I18n.t('components.templates.InventoryEditor.sku_description')}
            value={productID}
            onChangeText={this.onChangeProductId}
          />
          <View style={styles.quantityEditor}>
            {parseInt(quantity, 10) !== -1 ? (
              <View style={styles.textEditPart}>
                <TextInput
                  title={I18n.t('components.templates.InventoryEditor.quantity')}
                  editable={parseInt(quantity, 10) !== -1}
                  noBorder
                  value={`${quantity}`}
                  onChangeText={this.onChangeQuantity}
                  keyboardType="decimal-pad"
                />
              </View>
            ) : (
              <View style={styles.quantityPlaceholder}>
                <Text style={styles.label}>Quantity</Text>
                <Text style={styles.quantity}>Unlimited</Text>
              </View>
            )}
            <CheckBox
              title={I18n.t('components.templates.InventoryEditor.unlimited')}
              checked={parseInt(quantity, 10) === -1}
              onPress={this.onChangeUnlimited}
            />
          </View>
        </InputGroup>
        <Text style={styles.description}>
          {I18n.t('components.templates.InventoryEditor.quantity_sold_out')}          
        </Text>
      </React.Fragment>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(InventoryEditor);
