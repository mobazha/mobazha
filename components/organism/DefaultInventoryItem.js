import React from 'react';
import { View, Text } from 'react-native';
import { connect } from 'react-redux';
import { get, isEmpty } from 'lodash';

import ProductImage from '../atoms/ProductImage';
import TextInput from '../atoms/TextInput';
import { borderColor, primaryTextColor, greenColor, formLabelColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    borderWidth: 1,
    borderColor,
    flexDirection: 'row',
    alignItems: 'center',
  },
  content: {
    flexDirection: 'column',
    justifyContent: 'center',
    paddingHorizontal: 14,
  },
  title: {
    fontSize: 14,
    fontWeight: 'bold',
    color: primaryTextColor,
    marginBottom: 14,
  },
  titlePlaceholder: {
    fontSize: 14,
    fontStyle: 'italic',
    color: formLabelColor,
    marginBottom: 14,
  },
  price: {
    fontSize: 14,
    fontWeight: 'bold',
    color: greenColor,
  },
  editor: {
    borderBottomWidth: 1,
    borderBottomColor: '#e8e8e8',
    paddingHorizontal: 16,
  },
  quantityPlaceholder: {
    flexDirection: 'row',
    justifyContent: 'flex-start',
    paddingVertical: 20,
    flex: 1,
  },
  quantityEditor: {
    flexDirection: 'row',
    alignItems: 'center',
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
  quantity: {
    fontSize: 15,
    color: primaryTextColor,
    textAlign: 'left',
  },
  description: {
    paddingHorizontal: 16,
    fontSize: 13,
    color: '#8d8d8d',
    textAlign: 'left',
    marginTop: 12,
  },
};

class DefaultInventoryItem extends React.Component {
  constructor(props) {
    super(props);
    const productId = get(props, 'item.productId', '');
    const quantity = get(props, 'item.quantity', '-1');
    this.state = {
      productId,
      quantity,
    };
  }

  componentDidUpdate(prevProps) {
    const productId = get(this.props, 'item.productId', '');
    const quantity = get(this.props, 'item.quantity', '-1');
    const prevProductionId = get(prevProps, 'item.productId', '');
    const prevQuantity = get(prevProps, 'item.quantity', '-1');
    if (productId !== prevProductionId || quantity !== prevQuantity) {
      this.setState({ productId, quantity });
    }
  }

  onChangeSku = (productId) => {
    this.setState({ productId }, () => { this.props.onChange(this.state); });
  };

  onChangeQuantity = (quantity) => {
    this.setState({ quantity }, () => { this.props.onChange(this.state); });
  };

  onChangeUnlimited = () => {
    const { quantity } = this.state;
    this.setState({ quantity: parseInt(quantity, 10) === -1 ? '0' : '-1' }, () => { this.props.onChange(this.state); });
  };

  render() {
    const {
      title, images, price, isTracking, localLabelFromLocal,
    } = this.props;
    const { productId, quantity } = this.state;
    return (
      <React.Fragment>
        <View style={styles.wrapper}>
          <ProductImage images={images} />
          <View style={styles.content}>
            {isEmpty(title) ? (
              <Text style={styles.titlePlaceholder}>{I18n.t('components.organism.DefaultInventoryItem.no_title')}</Text>
            ) : (
              <Text style={styles.title}>{title}</Text>
            )}
            <Text style={styles.price}>
              {localLabelFromLocal(isEmpty(price) ? 0 : price)}
            </Text>
          </View>
        </View>
        <View style={styles.editor}>
          <TextInput
            title="SKU"
            placeholder={I18n.t('components.organism.DefaultInventoryItem.sku_info')}
            value={productId}
            onChangeText={this.onChangeSku}
          />
          {isTracking ? (
            <View style={styles.quantityEditor}>
              <TextInput
                title={I18n.t('components.organism.DefaultInventoryItem.quantity')}
                editable={parseInt(quantity, 10) !== -1}
                noBorder
                value={`${quantity}`}
                onChangeText={this.onChangeQuantity}
                keyboardType="decimal-pad"
              />
            </View>
          ) : (
            <View style={styles.quantityPlaceholder}>
              <Text style={styles.label}>{I18n.t('components.organism.DefaultInventoryItem.quantity')}</Text>
              <Text style={styles.quantity}>{I18n.t('components.organism.DefaultInventoryItem.unlimited')}</Text>
            </View>
          )}
        </View>
        <Text style={styles.description}>
          {
            isTracking ? I18n.t('components.organism.DefaultInventoryItem.quantity_sold_out')
            : I18n.t('components.organism.DefaultInventoryItem.quantity_unlimit')
          }
        </Text>
      </React.Fragment>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(DefaultInventoryItem);
