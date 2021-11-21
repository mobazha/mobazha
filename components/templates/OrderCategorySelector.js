import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableOpacity, ScrollView } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import orderStatusEn from '../../config/orderStatus.json';
import orderStatusZh from '../../config/zh/orderStatus.json';
import { primaryTextColor, secondaryTextColor, borderColor, staticLabelColor, formLabelColor } from '../commonColors';
import { OBLightModal } from './OBModal';
import Header from '../molecules/Header';
import NavCloseButton from '../atoms/NavCloseButton';
import { isOrderStatusInCategory } from '../../utils/order';

import {I18n} from '../../langs/I18n';
let orderStatus = orderStatusEn
if (I18n.locale.startsWith('zh')) {
  orderStatus = orderStatusZh
}

const styles = {
  optionTrigger: {
    marginHorizontal: 12,
    marginTop: 19,
    marginBottom: 15,
    paddingLeft: 15,
    paddingTop: 13,
    paddingRight: 12,
    paddingBottom: 14,
    borderWidth: 1,
    borderRadius: 2,
    borderColor,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  categoryTitleCount: {
    fontSize: 13,
    color: 'black',
  },
  categoryTitle: {
    fontWeight: 'bold',
  },
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
  count: {
    marginRight: 10,
    fontSize: 14,
    fontStyle: 'normal',
    textAlign: 'center',
    color: formLabelColor,
  },
  list: {
    paddingHorizontal: 16,
  },
};

const categorizeOrder = (orders, type) => {
  const categoryList = Object.keys(orderStatus)
    .filter(key => orderStatus[key].orderType.includes(type))
    .map(key => ({
      value: key,
      description: orderStatus[key].description,
      totalCount: orders.filter(order => isOrderStatusInCategory(order.state, key)).length,
      count: orders.filter(order => isOrderStatusInCategory(order.state, key) && !order.read).length,
    }));

  return [
    {
      value: 'All',
      description: I18n.t('components.templates.OrderCategorySelector.all', {type: I18n.t('components.templates.OrderCategorySelector.'+type)}),
      totalCount: orders.length,
      count: orders.filter(order => !order.read).length,
    },
    ...categoryList,
  ];
};

class OrderCategorySelector extends PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      showModal: false,
    };
  }

  handleShowModal = () => {
    this.setState({ showModal: true });
  };

  handleCategorySelected = (category) => {
    this.setState(
      { showModal: false },
      () => {
        this.props.onChange(category);
      },
    );
  }

  render() {
    const { orders, type, category } = this.props;
    const categories = categorizeOrder(orders, type);
    const { showModal } = this.state;
    const categoryObject = categories.find(c => c.value === category);
    return (
      <View>
        <TouchableOpacity style={styles.optionTrigger} onPress={this.handleShowModal}>
          <Text style={styles.categoryTitleCount}>
            <Text style={styles.categoryTitle}>{category == 'All'?I18n.t('components.templates.OrderCategorySelector.All'):category}</Text>
            {` (${categoryObject.totalCount})`}
          </Text>
          <View style={{ flex: 1 }} />
          <Ionicons
            name="ios-arrow-down"
            size={18}
            color={staticLabelColor}
          />
        </TouchableOpacity>
        <OBLightModal
          animationType="slide"
          transparent
          visible={showModal}
          onRequestClose={() => {}}
        >
          <Header
            modal
            left={<NavCloseButton />}
            onLeft={() => this.setState({ showModal: false })}
          />
          <ScrollView style={styles.list}>
            {categories.map((val, idx) => (
              <TouchableOpacity
                style={[styles.categoryWrapper, categories.length - 1 !== idx ? {} : styles.last]}
                key={`state_${val.value}`}
                onPress={() => this.handleCategorySelected(val.value)}
              >
                <View style={styles.descriptionWrapper}>
                  <Text style={styles.title}>{val.value== 'All'?I18n.t('components.templates.OrderCategorySelector.All'):val.value}</Text>
                  <Text style={styles.description}>{val.description}</Text>
                </View>
                <Text style={styles.count}>{val.totalCount}</Text>
              </TouchableOpacity>
            ))}
          </ScrollView>
        </OBLightModal>
      </View>
    );
  }
}

const mapStateToProps = state => ({});

const mapDispatchToProps = {};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(OrderCategorySelector);
