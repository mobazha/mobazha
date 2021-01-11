import React from 'react';
import { View, Text, ScrollView } from 'react-native';
import { connect } from 'react-redux';
import { get } from 'lodash';

import NavCloseButton from '../atoms/NavCloseButton';
import LinkText from '../atoms/LinkText';
import InputGroup from '../atoms/InputGroup';
import TextInput from '../atoms/TextInput';
import Header from '../molecules/Header';
import { OBLightModal } from './OBModal';
import { linkTextColor, warningColor, borderColor } from '../commonColors';
import { getHashCode } from '../../api/hash';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
  },
  errorText: {
    color: warningColor,
    fontSize: 16,
    padding: 12,
  },
  couponHighlight: {
    fontWeight: 'bold',
  },
  textInputWrapper: {
    borderStyle: 'solid',
    borderBottomWidth: 1,
    borderTopWidth: 1,
    borderColor,
  },
  couponInput: {
    marginTop: 8,
    borderStyle: 'solid',
    borderColor,
    borderTopWidth: 1,
  },
};

export default class CouponApplyModal extends React.Component {
  state = {
    couponCode: '',
    invalid: false,
  }

  componentDidUpdate(prevProps) {
    const { visible } = this.props;
    if (!visible && prevProps.visible) {
      this.setState({ couponCode: '', invalid: false });
    }
  }

  handleChangeCoupon = (couponCode) => {
    this.setState({ couponCode, invalid: false });
  }

  handleSubmit = async () => {
    const { listingDetail = {}, onSuccess, hideModal } = this.props;
    const { couponCode } = this.state;
    const { coupons = [] } = listingDetail;
    try {
      const { hash: couponHash } = await getHashCode(couponCode);
      const couponDetail = coupons.find(cp => (cp.discountCode === couponCode || cp.hash === couponHash));
      if (couponDetail) {
        const percentDisc = get(couponDetail, 'percentDiscount', 0);
        const priceDis = get(couponDetail, 'priceDiscount', 0);
        const itemPrice = get(listingDetail, 'item.price');
        const totalDis = itemPrice * percentDisc * 0.01 + priceDis;
        if (totalDis < itemPrice) {
          hideModal();
          onSuccess({ couponCode, couponHash });
          return;
        }
      }
    } finally {
      this.setState({ invalid: true });
    }
  }

  render() {
    const { hideModal, visible } = this.props;
    const { couponCode, invalid } = this.state;
    return (
      <OBLightModal visible={visible} animationType="slide">
        <ScrollView bounces={false} keyboardShouldPersistTaps="always">
          <Header
            modal
            left={<NavCloseButton />}
            onLeft={hideModal}
            right={<LinkText text={I18n.t('components.templates.CouponApplyModal.apply')} color={linkTextColor} fontSize={16} />}
            onRight={this.handleSubmit}
          />
          <View style={styles.wrapper}>
            <InputGroup title={I18n.t('components.templates.CouponApplyModal.enter_coupon')} contentStyle={styles.couponInput}>
              <TextInput
                noTitle
                noBorder
                value={couponCode}
                placeholder={I18n.t('components.templates.CouponApplyModal.enter_hint')}
                onChangeText={this.handleChangeCoupon}
                autoFocus
              />
            </InputGroup>
            {invalid && (
              <Text style={styles.errorText}>
                {I18n.t('components.templates.CouponApplyModal.coupon')} "<Text style={styles.couponHighlight}>{couponCode}</Text>" {I18n.t('components.templates.CouponApplyModal.not_valid')}
              </Text>
            )}
          </View>
        </ScrollView>
      </OBLightModal>
    );
  }
}
