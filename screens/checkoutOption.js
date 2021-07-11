import React, { PureComponent } from 'react';
import { View, Alert, ScrollView } from 'react-native';
import { isEmpty, isEqual } from 'lodash';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import NavBackButton from '../components/atoms/NavBackButton';
import { screenWrapper } from '../utils/styles';
import { linkTextColor } from '../components/commonColors';
import CheckoutHeader from '../components/organism/CheckoutHeader';
import VariantOptions from '../components/organism/VariantOptions';

import {I18n} from '../langs/I18n';
class CheckoutOption extends PureComponent {
  constructor(props) {
    super(props);

    const listing = props.navigation.getParam('listing');
    const { options } = listing.listing.item;

    this.state = {
      combo: options.map(() => -1),
      quantity: 1,
    };
  }

  checkOptionSelected() {
    const { combo } = this.state;
    if (combo.find(c => c === -1)) {
      return false;
    }
    return true;
  }

  handleGoToNext = () => {
    const listing = this.props.navigation.getParam('listing');
    const { options } = listing.listing.item;
    const { combo, quantity } = this.state;
    if (this.checkOptionSelected() || isEmpty(options)) {
      this.props.navigation.navigate({
        routeName: 'CheckoutListing',
        params: {
          listing,
          combo,
          quantity,
          screenKey: this.props.navigation.state.key,
        },
      });
    } else {
      Alert.alert('Please select the variants');
    }
  }

  goBack = () => {
    this.props.navigation.goBack();
  };

  handleChange = key => (value) => {
    const dict = {};
    dict[key] = value;
    this.setState({ ...dict });
  };

  render() {
    const listing = this.props.navigation.getParam('listing');
    const { options, skus } = listing.listing.item;

    const { combo, quantity } = this.state;
    const variantInfo = skus.find(o => isEqual(o.variantCombo, combo));

    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.goBack}
          title={I18n.t('screens.checkoutOption.Checkout')}
          right={<LinkText text="Next" color={linkTextColor} />}
          onRight={this.handleGoToNext}
        />
        <CheckoutHeader
          listing={listing}
          quantity={quantity}
          variantInfo={variantInfo}
          onChange={this.handleChange('quantity')}
        />
        {skus.length > 1 && (
          <ScrollView style={{ flex: 1 }}>
            <VariantOptions
              value={combo}
              onChange={this.handleChange('combo')}
              variants={skus}
              options={options}
            />
          </ScrollView>
        )}
      </View>
    );
  }
}

export default CheckoutOption;
