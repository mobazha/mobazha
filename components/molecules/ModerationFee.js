import React, { PureComponent } from 'react';
import { View, Dimensions, Text, Platform } from 'react-native';
import { hasIn } from 'lodash';
import { TabViewAnimated, TabBar, SceneMap, TabViewPagerScroll, TabViewPagerPan } from 'react-native-tab-view';

import { primaryTextColor } from '../commonColors';
import InputGroup from '../atoms/InputGroup';
import TextInput from '../atoms/TextInput';

import {I18n} from '../../langs/I18n';

const styles = {
  initialLayout: {
    height: 150,
    width: Dimensions.get('window').width,
  },
  tabContentWrapper: {
    paddingHorizontal: 16,
    height: 'auto',
  },
  activeHeader: {
    backgroundColor: 'white',
    borderTopWidth: 1,
    borderBottomWidth: 1,
    borderColor: 'rgb(221,221,226)',
    paddingVertical: 10,
    width: '100%',
  },
  deactiveHeader: {
    backgroundColor: 'rgb(243, 243, 243)',
    borderTopWidth: 1,
    borderBottomWidth: 1,
    borderColor: 'rgb(221,221,226)',
    justifyContent: 'center',
    alignItems: 'center',
    paddingVertical: 10,
    width: '100%',
  },
  tabTitle: {
    fontSize: 14,
    color: primaryTextColor,
    textAlign: 'center',
  },
  tabStyle: {
    padding: 0,
    backgroundColor: 'white',
  },
  tabBarStyle: {
    backgroundColor: 'white',
    zIndex: 0,
    elevation: 0,
  },
  indicatorStyle: {
    backgroundColor: 'rgb(234, 234, 234)',
  },
};

const renderHeader = props => (
  <TabBar
    {...props}
    renderLabel={(props) => {
      const { focused, route: { title } } = props;
      return (
        <View style={focused ? styles.activeHeader : styles.deactiveHeader}>
          <Text style={styles.tabTitle}>
            {title}
          </Text>
        </View>
      );
    }}
    tabStyle={styles.tabStyle}
    style={styles.tabBarStyle}
    indicatorStyle={styles.indicatorStyle}
  />
);

const renderPager = props => (
  (Platform.OS === 'ios') ?
    <TabViewPagerScroll {...props} /> : <TabViewPagerPan {...props} />
);

export default class ModerationFee extends PureComponent {
  state = {
    feeType: 'FIXED',
    fixedAmount: '',
    percentage: '',
    tabNav: {
      index: 0,
      routes: [
        { key: 'fixed', title: 'Flat Fee' },
        { key: 'percentage', title: 'Percentage(%)' },
        { key: 'fixed_p_per', title: 'Flat Fee + (%)' },
      ],
    },
  };

  onChange() {
    let fee = {};
    const { feeType, fixedAmount, percentage } = this.state;
    switch (feeType) {
      case 'FIXED':
        fee = {
          feeType,
          fixedFee: {
            currencyCode: 'USD',
            amount: fixedAmount,
          },
        };
        break;
      case 'PERCENTAGE':
        fee = {
          feeType,
          percentage,
        };
        break;
      default:
        fee = {
          feeType,
          fixedFee: {
            currencyCode: 'USD',
            amount: fixedAmount,
          },
          percentage,
        };
        break;
    }
    this.props.onChange(fee);
  }

  handleIndexChange(idx) {
    const tabNav = { ...this.state.tabNav };
    tabNav.index = idx;
    this.setState({
      tabNav,
    });
  }

  percentageInput() {
    const { fee } = this.props;
    const orgPercentage = hasIn(fee, 'percentage') ? fee.percentage : '';
    return (
      <View style={[styles.tabContentWrapper, { height: 48 }]}>
        <TextInput
          noBorder
          title={I18n.t('components.molecules.ModerationFee.percentage')}
          keyboardType="numeric"
          defaultValue={orgPercentage}
          onChangeText={(percentage) => {
            this.setState({
              feeType: 'PERCENTAGE',
              percentage,
            }, () => {
              this.onChange();
            });
          }}
        />
      </View>
    );
  }

  fixedPercentage() {
    const { fee } = this.props;
    const orgFixedAmount = hasIn(fee, 'fixedFee.amount') ? fee.fixedFee.amount : '';
    const orgPercentage = hasIn(fee, 'percentage') ? fee.percentage : '';
    return (
      <View style={styles.tabContentWrapper}>
        <TextInput
          title={I18n.t('components.molecules.ModerationFee.flat_fee')}
          keyboardType="numeric"
          defaultValue={orgFixedAmount}
          onChangeText={(fixedFee) => {
            this.setState({
              fixedType: 'FIXED_PLUS_PERCENTAGE',
              fixedAmount: fixedFee,
            }, () => {
              this.onChange();
            });
          }}
        />
        <TextInput
          noBorder
          title={I18n.t('components.molecules.ModerationFee.percentage')}
          keyboardType="numeric"
          defaultValue={orgPercentage}
          onChangeText={(percentage) => {
            this.setState({
              fixedType: 'FIXED_PLUS_PERCENTAGE',
              percentage,
            }, () => {
              this.onChange();
            });
          }}
        />
      </View>
    );
  }

  fixedInput() {
    const { fee } = this.props;
    const orgFixedAmount = hasIn(fee, 'fixedFee.amount') ? fee.fixedFee.amount : '';
    return (
      <View style={[styles.tabContentWrapper, { height: 48 }]}>
        <TextInput
          noBorder
          title="Fee ($)"
          title={I18n.t('components.molecules.ModerationFee.fee')}
          keyboardType="numeric"
          defaultValue={orgFixedAmount}
          onChangeText={(fee) => {
            this.setState({
              feeType: 'FIXED',
              fixedAmount: fee,
            }, () => {
              this.onChange();
            });
          }}
        />
      </View>
    );
  }

  renderControl = SceneMap({
    fixed: () => this.fixedInput(),
    percentage: () => this.percentageInput(),
    fixed_p_per: () => this.fixedPercentage(),
  })

  render() {
    return (
      <InputGroup title="Fees" noPadding>
        <TabViewAnimated
          navigationState={this.state.tabNav}
          renderScene={this.renderControl}
          renderPager={renderPager}
          renderHeader={renderHeader}
          onIndexChange={(idx) => { this.handleIndexChange(idx); }}
          initialLayout={styles.initialLayout}
        />
      </InputGroup>
    );
  }
}
