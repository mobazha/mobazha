import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View } from 'react-native';

import Header from '../components/molecules/Header';
import CouponsTemplate from '../components/templates/CouponsTemplate';
import NavBackButton from '../components/atoms/NavBackButton';
import CouponModal from '../components/templates/CouponModal';

import { addCoupon, removeCoupon, editCoupon } from '../reducers/createListing';
import { getCurrencySymbol } from '../utils/currency';
import { screenWrapper } from '../utils/styles';

import {I18n} from '../langs/I18n';
class AddListingCoupon extends PureComponent {
  state = {
    mode: 'new',
    showCouponModal: false,
    selectedCoupon: -1,
  };

  onLeft = () => {
    this.props.navigation.goBack();
  };

  onAdd = () => {
    this.setState(
      {
        mode: 'new',
        selectedCoupon: -1,
      },
      () => {
        this.setState({
          showCouponModal: true,
        });
      },
    );
  };

  onEdit = (coupon) => {
    this.setState({
      mode: 'edit',
      showCouponModal: true,
      selectedCoupon: coupon,
    });
  };

  onSave = (value) => {
    const { selectedCoupon } = this.state;
    if (selectedCoupon === -1) {
      this.props.addCoupon(value);
    } else {
      this.props.editCoupon({
        selectedCoupon,
        value,
      });
      this.setState({
        selectedCoupon: -1,
      });
    }
  };

  onRemove = (couponCode) => {
    this.props.removeCoupon(couponCode);
  };

  getDefaultValue = () => {
    const { selectedCoupon } = this.state;
    const { coupons } = this.props;
    if (selectedCoupon === -1) {
      return {
        title: '',
        discountCode: '',
        percentDiscount: '0',
      };
    }
    return {
      ...coupons[selectedCoupon],
    };
  };

  hideModal = () => {
    this.setState({
      showCouponModal: false,
    });
  };

  render() {
    const { coupons, localCurrency, price } = this.props;
    const { mode, showCouponModal } = this.state;
    const localSymbol = getCurrencySymbol(localCurrency);
    const defaultValue = this.getDefaultValue();
    return (
      <View style={screenWrapper.wrapper}>
        <Header left={<NavBackButton />} onLeft={this.onLeft} title={I18n.t('screens.addListingCoupon.Coupons')} />
        <CouponsTemplate
          coupons={coupons}
          localSymbol={localSymbol}
          onAdd={this.onAdd}
          onEdit={this.onEdit}
          onRemove={this.onRemove}
        />
        <CouponModal
          mode={mode}
          defaultValue={defaultValue}
          price={price}
          localCurrency={localCurrency}
          visible={showCouponModal}
          hideModal={this.hideModal}
          onSave={this.onSave}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  price: state.createListing.price,
  coupons: state.createListing.coupons,
  localCurrency: state.settings.localCurrency,
});

const mapDispatchToProps = {
  addCoupon,
  editCoupon,
  removeCoupon,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(AddListingCoupon);
