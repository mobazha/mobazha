import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import { connect } from 'react-redux';

import { borderColor, greenColor, secondaryTextColor, primaryTextColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    borderTopWidth: 1,
    borderTopColor: '#e8e8e8',
    paddingVertical: 16,
  },
  lastItem: {
    borderBottomWidth: 1,
    borderBottomColor: '#e8e8e8',
  },
  content: {
    flexDirection: 'column',
    justifyContent: 'space-between',
    paddingHorizontal: 14,
  },
  variantsHolder: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 12,
  },
  variant: {
    borderWidth: 1,
    borderColor,
    paddingVertical: 7,
    paddingHorizontal: 11,
    fontSize: 13,
    fontWeight: 'bold',
    marginRight: 6,
    color: primaryTextColor,
  },
  price: {
    fontSize: 14,
    fontWeight: 'bold',
    color: greenColor,
    marginBottom: 12,
  },
  quantity: {
    fontSize: 13,
    color: secondaryTextColor,
  },
};

const getVariantList = (options, variantCombo) =>
  variantCombo.map((variant, idx) => options[idx].variants[variant]);

class InventoryItem extends React.Component {
  render() {
    const {
      item: { variantCombo, quantity, surcharge },
      options,
      price,
      localSymbol,
      isLast,
      onPress,
    } = this.props;
    const variants = getVariantList(options, variantCombo);
    return (
      <TouchableWithoutFeedback
        onPress={onPress}
      >
        <View style={[styles.wrapper, isLast ? styles.lastItem : {}]}>
          <View style={styles.content}>
            <View style={styles.variantsHolder}>
              {variants.map((val, idx) => (
                <Text style={styles.variant} key={`variant_${idx}`}>
                  {val}
                </Text>
              ))}
            </View>
            <Text style={styles.price}>
              {localSymbol}
              {parseFloat(price || '0') + parseFloat(surcharge || '0')}
            </Text>
            <Text style={styles.quantity}>QTY: {parseInt(quantity, 10) === -1 ? I18n.t('components.organism.InventoryItem.unlimited'): quantity}</Text>
          </View>
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  options: state.createListing.options,
  price: state.createListing.price,
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(InventoryItem);
