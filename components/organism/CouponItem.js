import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import { connect } from 'react-redux';

import OptionButton from '../atoms/NavOptionButton';
import { convertorsMap } from '../../selectors/currency';
import { getFixedCurrency } from '../../utils/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'column',
    alignItems: 'center',
    borderBottomWidth: 1,
    borderColor: '#e8e8e8',
    padding: 20,
  },
  titlePart: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 20,
    width: '100%',
  },
  detailPart: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-start',
    width: '100%',
  },
  title: {
    fontSize: 15,
    fontWeight: 'bold',
    color: '#000',
  },
  optionButton: {
    paddingLeft: 10,
  },
  couponCode: {
    fontSize: 13,
    fontWeight: 'bold',
    paddingVertical: 7,
    paddingHorizontal: 20,
    textAlign: 'center',
    color: '#000',
    borderWidth: 1,
    borderRadius: 2,
    borderColor: '#c8c7cc',
    marginRight: 15,
  },
  discountText: {
    color: '#00BF65',
    fontSize: 14,
    fontWeight: 'bold',
  },
};

class CouponItem extends React.Component {
  onPress = () => {
    const { discountCode } = this.props;
    this.props.onPress(discountCode);
  };

  getDiscountText = () => {
    const { percentDiscount, priceDiscount, currencySymbol, localCurrency, convertCurrency } = this.props;
    if (percentDiscount) {
      return `${percentDiscount}% off`;
    }
    return `${currencySymbol}${convertCurrency(priceDiscount, localCurrency).toFixed(2)} off`;
  };

  render() {
    const { title, discountCode } = this.props;
    const discountText = this.getDiscountText();
    return (
      <TouchableWithoutFeedback onPress={this.onPress}>
        <View style={styles.wrapper}>
          <View style={styles.titlePart}>
            <Text style={styles.title}>{title || discountText}</Text>
            <OptionButton />
          </View>
          <View style={styles.detailPart}>
            <Text style={styles.couponCode}>{discountCode}</Text>
            <Text style={styles.discountText}>{discountText}</Text>
          </View>
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => convertorsMap(state);

export default connect(mapStateToProps)(CouponItem);
