import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert } from 'react-native';
import { hasIn, remove, isEmpty, get } from 'lodash';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import InputGroup from '../components/atoms/InputGroup';
import TextInput from '../components/atoms/TextInput';
import MultiSelector from '../components/organism/MultiSelector';
import ShippingPrices from '../components/organism/ShippingPrices';

import { addShippingOption, updateShippingOption } from '../reducers/createListing';

import countryList from '../config/countries.json';
import NavBackButton from '../components/atoms/NavBackButton';

import { convertorsMap } from '../selectors/currency';

import { screenWrapper } from '../utils/styles';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';
class AddShippingMethod extends PureComponent {
  constructor(props) {
    super(props);
    const { shippingOptions } = this.props;
    if (hasIn(props.navigation.state.params, 'idx')) {
      const idx = props.navigation.getParam('idx');
      const selectableCountry = [{ label: '🌏 Worldwide', value: 'ALL' }, ...countryList];
      shippingOptions.forEach((val, i) => {
        if (idx !== i) {
          val.regions.forEach((region) => { remove(selectableCountry, o => o.value === region.value); });
        }
      });
      this.state = {
        name: shippingOptions[idx].name,
        type: shippingOptions[idx].type,
        selectableCountry,
        regions: [...shippingOptions[idx].regions],
        services: [...shippingOptions[idx].services],
      };
    } else {
      const selectableCountry = [{ label: '🌏 Worldwide', value: 'ALL' }, ...countryList];
      shippingOptions.forEach((val) => {
        val.regions.forEach((region) => { remove(selectableCountry, o => o.value === region.value); });
      });
      this.state = {
        name: '',
        type: 'FIXED_PRICE',
        selectableCountry,
        regions: [],
        services: [],
      };
    }
  }

  onLeft = () => {
    this.props.navigation.goBack();
  };

  onRight = () => {
    const {
      name, type, regions, services,
    } = this.state;
    const { navigation, updateShippingOption, addShippingOption } = this.props;
    const serviceValidFlgs = this.validateServices();
    if (regions.length > 0) {
      if (name.length <= 40) {
        if (serviceValidFlgs[0]) {
          if (!isEmpty(services)) {
            if (hasIn(navigation.state.params, 'idx')) {
              const idx = navigation.getParam('idx');
              updateShippingOption({
                idx,
                shippingOption: {
                  name, type, regions, services,
                },
              });
              eventTracker.trackEvent('CreateListing-UpdatedShippingOption');
            } else {
              eventTracker.trackEvent('CreateListing-CreatedShippingOption');
              addShippingOption({
                name, type, regions, services,
              });
            }
            navigation.goBack();
          } else {
            Alert.alert(I18n.t('screens.addShippingMethod.fill_required'));
          }
        } else if (serviceValidFlgs[1]) {
          Alert.alert(I18n.t('screens.addShippingMethod.must_be_less'));
        } else {
          Alert.alert(I18n.t('screens.addShippingMethod.fill_required'));
        }
      } else {
        Alert.alert(I18n.t('screens.addShippingMethod.must_be_less'));
      }
    } else {
      Alert.alert(I18n.t('screens.addShippingMethod.select_destination'));
    }
  };

  onChangeName = (name) => {
    this.setState({ name });
  }

  onChangeRegions = (regions) => {
    this.setState({ regions });
  }

  onChangeServices = (services) => {
    this.setState({ services });
  }

  validateServices = () => {
    const { services } = this.state;
    let lengthFlg = false;
    const isValid = services.reduce((prev, current) => {
      if (prev === false) {
        return false;
      }
      if (get(current, 'name.length', 0) > 40) {
        lengthFlg = true;
      }
      return !isEmpty(current.name) &&
        (!isEmpty(current.price) || current.price > 0) &&
        !isEmpty(current.estimatedDelivery) &&
        current.name.length <= 40;
    }, true);
    return [isValid, lengthFlg];
  };

  render() {
    const { regions, name, selectableCountry } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.onLeft}
          title= {I18n.t('screens.addShippingMethod.add_shipping_option')}
          right={<LinkText text={I18n.t('screens.addShippingMethod.save')} />}
          onRight={this.onRight}
        />
        <InputGroup title={I18n.t('screens.addShippingMethod.shipping_option')}>
          <TextInput
            title= {I18n.t('screens.addShippingMethod.title')}
            required
            value={name}
            onChangeText={this.onChangeName}
            onChange={this.onChangeName}
            placeholder={I18n.t('screens.addShippingMethod.option_description')}
          />
          <MultiSelector
            title={I18n.t('screens.addShippingMethod.destinations')}
            required
            selection={regions}
            options={selectableCountry}
            onChange={this.onChangeRegions}
          />
        </InputGroup>
        <ShippingPrices
          services={this.state.services}
          onChange={this.onChangeServices}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  shippingOptions: state.createListing.shippingOptions,
});

const mapDispatchToProps = {
  addShippingOption,
  updateShippingOption,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(AddShippingMethod);
