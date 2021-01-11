import React from 'react';
import { View, FlatList, Alert } from 'react-native';

import MoreButton from '../atoms/MoreButton';
import CouponItem from '../organism/CouponItem';
import EmptyCoupons from '../organism/EmptyCoupons';

import OBActionSheet from '../organism/ActionSheet';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flex: 1,
  },
  list: {
    flex: 1,
  },
  moreButtonWrapper: {
    paddingVertical: 20,
    paddingLeft: 15,
  },
};

const actionList = ['Edit', 'Delete', 'Cancel'];

export default class CouponsTemplate extends React.Component {
  state = {
    selectedCoupon: '',
  };

  onClickCoupon = (id) => {
    this.setState({
      selectedCoupon: id,
    });
    this.actionSheet.show();
  };

  setActionSheet = (ref) => {
    this.actionSheet = ref;
  };

  handleChange = (index) => {
    const { selectedCoupon } = this.state;
    switch (index) {
      case 0:
        this.props.onEdit(selectedCoupon);
        break;
      case 1:
        this.confirmRemove();
        break;
      default:
        break;
    }
  };

  confirmRemove = () => {
    const { selectedCoupon } = this.state;
    Alert.alert('Delete Coupon?', "You can't undo this action", [
      { text: 'Cancel' },
      { text: 'Delete', onPress: () => this.props.onRemove(selectedCoupon) },
    ]);
  };

  renderItem = ({ item, index }) => {
    const { localSymbol } = this.props;
    return (
      <CouponItem
        {...item}
        currencySymbol={localSymbol}
        onPress={() => {
          this.onClickCoupon(index);
        }}
      />
    );
  };

  render() {
    const { coupons } = this.props;
    return (
      <View style={styles.wrapper}>
        <FlatList
          contentContainerStyle={styles.list}
          data={coupons}
          keyExtractor={(item, index) => `coupon_${index}`}
          renderItem={this.renderItem}
          ListEmptyComponent={<EmptyCoupons onAdd={this.props.onAdd} />}
          ListFooterComponent={
            coupons.length > 0 && (
              <View style={styles.moreButtonWrapper}>
                <MoreButton title="Add coupon" onPress={this.props.onAdd} />
              </View>
            )
          }
          extraData={coupons}
        />
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={actionList}
          cancelButtonIndex={2}
        />
      </View>
    );
  }
}
