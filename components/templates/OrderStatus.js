import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import orderStatusEn from '../../config/orderStatus.json';
import orderStatusZh from '../../config/zh/orderStatus.json';
import { getStatusId } from '../../utils/order';
import { primaryTextColor, secondaryTextColor, highlightColor, borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';
let orderStatus = orderStatusEn
if (I18n.locale.startsWith('zh')) {
  orderStatus = orderStatusZh
}


const styles = {
  wrapper: {},
  categoryWrapper: {
    flexDirection: 'row',
    paddingVertical: 13,
    borderBottomWidth: 1,
    borderColor,
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  last: {
    borderColor: 'transparent',
  },
  descriptionWrapper: {
    flexDirection: 'column',
  },
  description: {
    fontSize: 14,
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  title: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  rightPart: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  countWrapper: {
    width: 24,
    height: 24,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: highlightColor,
    marginRight: 10,
  },
  count: {
    fontSize: 12,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: 'white',
  },
};

const categorizeOrder = (orders, type) => {
  const result = [];
  result[0] = {
    label: 'All',
    value: 'ALL',
    description: `All ${type}`,
    count: orders.length,
    totalCount: orders.length,
  };
  orderStatus.map((val, idx) => {
    result[idx + 1] = {
      label: val.label,
      value: val.value,
      description: val.description,
      totalCount: 0,
      count: 0,
    };
  });
  let totalUnread = 0;
  orders.map((val) => {
    const statusId = getStatusId(val.state);
    result[statusId + 1].totalCount += 1;
    if (!val.read) {
      totalUnread += 1;
      result[statusId + 1].count += 1;
    }
  });
  result[0].count = totalUnread;
  return result;
};

class OrderStatus extends PureComponent {
  render() {
    const { orders, type, toOrderCategory } = this.props;
    const categories = categorizeOrder(orders, type);
    return (
      <View style={styles.wrapper}>
        {categories.map((val, idx) => (
          <TouchableWithoutFeedback
            key={`state_${val.value}`}
            onPress={() => {
              toOrderCategory(val.value);
            }}
          >
            <View
              style={[
                styles.categoryWrapper,
                categories.length - 1 !== idx ? {} : styles.last
              ]}
            >
              <View style={styles.descriptionWrapper}>
                <Text style={styles.title}>
                  {val.label} {val.totalCount > 0 ? `(${val.totalCount})` : ''}
                </Text>
                <Text style={styles.description}>{val.description}</Text>
              </View>
              <View style={styles.rightPart}>
                {val.count > 0 && (
                  <View style={styles.countWrapper}>
                    <Text style={styles.count}>{val.count}</Text>
                  </View>
                )}
                <Ionicons name="ios-arrow-forward" size={20} color={primaryTextColor} />
              </View>
            </View>
          </TouchableWithoutFeedback>
        ))}
      </View>
    );
  }
}

const mapStateToProps = state => ({});

const mapDispatchToProps = {};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(OrderStatus);
