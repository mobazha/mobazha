import React from 'react';
import { View, TouchableWithoutFeedback, Alert, ScrollView } from 'react-native';
import { get, hasIn, isEmpty } from 'lodash';
import deepEqual from 'deep-equal';
import { connect } from 'react-redux';

import NavCloseButton from '../atoms/NavCloseButton';
import LinkText from '../atoms/LinkText';
import InputGroup from '../atoms/InputGroup';
import TextInput from '../atoms/TextInput';
import Header from '../molecules/Header';
import { OBLightModal } from './OBModal';
import { linkTextColor } from '../commonColors';

import { parseCoupon } from '../../utils/dataMapping';
import { getPriceInMinimumUnit, getFixedCurrency } from '../../utils/currency';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
  },
  discountWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  textInputWrapper: {
    flex: 1,
  },
};

class CouponModal extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      discountMode: hasIn(props, 'defaultValue.percentDiscount') ? 'percentage' : 'currency',
      value: props.defaultValue,
    };
  }

  componentDidUpdate(prevProps) {
    const { defaultValue, convertCurrency, localCurrency } = this.props;
    if (!deepEqual(defaultValue, prevProps.defaultValue)) {
      const value = { ...defaultValue };
      value.priceDiscount = value.priceDiscount ?
        convertCurrency(value.priceDiscount, localCurrency).toFixed(2) : '';
      this.setState({
        discountMode: defaultValue.percentDiscount ? 'percentage' : 'currency',
        value,
      });
    }
    if (this.props.mode === 'new' && this.props.visible && !prevProps.visible) {
      this.setState({
        discountMode: 'percentage',
        value: {
          title: '',
          discountCode: '',
          percentDiscount: '',
        },
      });
    }
  }

  onChangeDiscountMode = () => {
    const {
      discountMode,
      value: { discountCode, title },
    } = this.state;
    if (discountMode === 'percentage') {
      this.setState({
        discountMode: 'currency',
        value: {
          discountCode,
          title,
          priceDiscount: '',
        },
      });
    } else {
      this.setState({
        discountMode: 'percentage',
        value: {
          discountCode,
          title,
          percentDiscount: '',
        },
      });
    }
  };

  onChangeDiscountCode = (discountCode) => {
    const { value } = this.state;
    this.setState({
      value: {
        ...value,
        discountCode,
      },
    });
  };

  onChangeTitle = (title) => {
    const { value } = this.state;
    this.setState({
      value: {
        ...value,
        title,
      },
    });
  };

  onChangeDiscountAmount = (formatedAmount) => {
    const { value, discountMode } = this.state;
    this.setState({
      value: {
        title: value.title,
        discountCode: value.discountCode,
        ...(discountMode === 'percentage'
          ? { percentDiscount: formatedAmount }
          : { priceDiscount: formatedAmount }),
      },
    });
  };

  onSave = () => {
    const { price, localCurrency } = this.props;
    const { value, discountMode } = this.state;
    if (isEmpty(value.title)) {
      Alert.alert('Discount title cannot be empty');
      return;
    }

    const parsedValue = parseCoupon(value);
    if (isEmpty(parsedValue.discountCode)) {
      Alert.alert('Discount code cannot be empty');
      return;
    }

    const { percentDiscount, priceDiscount } = parsedValue;

    if (discountMode === 'percentage') {
      if (!percentDiscount || percentDiscount > 99 || percentDiscount < 1) {
        Alert.alert('Sorry, but the value must be between 1 and 99.');
        return;
      }
    } else {
      const discountPrice = getPriceInMinimumUnit(priceDiscount, localCurrency);
      const priceInMinimumUnit = getPriceInMinimumUnit(price, localCurrency);
      if (!discountPrice) {
        Alert.alert('Discount value cannot be empty');
        return;
      }

      if (discountPrice >= priceInMinimumUnit) {
        Alert.alert('Sorry, but the discount exceeds the value of the item.');
        return;
      }
      parsedValue.priceDiscount = parseFloat(discountPrice);
    }
    this.props.onSave(parsedValue);
    this.props.hideModal();
  };

  render() {
    const {
      mode, hideModal, visible, localCurrency, localMask,
    } = this.props;
    const { discountMode, value } = this.state;
    return (
      <OBLightModal visible={visible} animationType="slide">
        <Header
          modal
          title={mode === 'edit' ? I18n.t('components.templates.CouponModal.edit_coupon') : I18n.t('components.templates.CouponModal.new_coupon')}
          left={<NavCloseButton />}
          onLeft={hideModal}
          right={<LinkText text={I18n.t('components.templates.CouponModal.save')} color={linkTextColor} fontSize={16} />}
          onRight={this.onSave}
        />
        <ScrollView style={styles.wrapper}>
          <InputGroup>
            <TextInput
              title={I18n.t('components.templates.CouponModal.title')}
              required
              value={value.title}
              onChangeText={this.onChangeTitle}
              placeholder={I18n.t('components.templates.CouponModal.title_hint')}
            />
            <TextInput
              title={I18n.t('components.templates.CouponModal.code')}
              required
              value={value.discountCode}
              onChangeText={this.onChangeDiscountCode}
              placeholder={I18n.t('components.templates.CouponModal.code_hint')}
            />
            <View style={styles.discountWrapper}>
              <View style={styles.textInputWrapper}>
                <TextInput
                  noBorder
                  title={I18n.t('components.templates.CouponModal.discount')}
                  required
                  onChangeText={this.onChangeDiscountAmount}
                  value={
                    discountMode === 'percentage'
                      ? `${get(value, 'percentDiscount', 0)}`
                      : `${get(value, 'priceDiscount', 0)}`
                  }
                  placeholder={discountMode === 'percentage' ? I18n.t('components.templates.CouponModal.discount_hint1') : I18n.t('components.templates.CouponModal.discount_hint2')}
                  mask={discountMode === 'percentage' ? '[990]{.}[99]%' : localMask}
                  keyboardType="decimal-pad"
                />
              </View>
              <TouchableWithoutFeedback onPress={this.onChangeDiscountMode}>
                <View>
                  <LinkText
                    text={discountMode === 'percentage' ? I18n.t('components.templates.CouponModal.percent') : localCurrency}
                    color={linkTextColor}
                    fontSize={14}
                  />
                </View>
              </TouchableWithoutFeedback>
            </View>
          </InputGroup>
        </ScrollView>
      </OBLightModal>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(CouponModal);
