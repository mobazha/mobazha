import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { View, Alert, findNodeHandle } from 'react-native';
import { NavigationActions } from 'react-navigation';
import { KeyboardAwareScrollView } from 'react-native-keyboard-aware-scroll-view';
import * as _ from 'lodash';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import InputGroup from '../components/atoms/InputGroup';
import TextInput from '../components/atoms/TextInput';
import TextArea from '../components/atoms/TextArea';
import { patchSettingsRequest } from '../reducers/settings';
import { setShippingAddress } from '../reducers/appstate';

import { screenWrapper } from '../utils/styles';

import countries from '../config/countries.json';
import NavBackButton from '../components/atoms/NavBackButton';
import { linkTextColor } from '../components/commonColors';
import RadioModalFilter from '../components/molecules/RadioModalFilter';
import { eventTracker } from '../utils/EventTracker';

import {I18n} from '../langs/I18n';
class EditShippingAddress extends PureComponent {
  constructor(props) {
    super(props);
    const { shippingAddresses } = this.props;
    const shippingIndex = props.navigation.getParam('shippingIndex');
    if (shippingIndex === -1) {
      this.state = {
        addressLineOne: '',
        addressLineTwo: '',
        company: '',
        city: '',
        country: '',
        name: '',
        state: '',
        addressNotes: '',
      };
    } else {
      const shippingAddress = shippingAddresses[shippingIndex];
      this.state = { ...shippingAddress };
    }
  }

  componentDidMount() {
    this.nameInput.focus();
  }

  onChange = field => (value) => {
    const updateObject = {};
    _.set(updateObject, field, value);
    this.setState(updateObject);
  };

  setRef = control => (ref) => { _.set(this, control, ref); };

  setFocus = control => () => { _.get(this, control).focus(); };

  findNodePos = (event) => { this.scrollToInput(findNodeHandle(event.target)); };

  submitData = () => {
    const {
      navigation, shippingAddresses, setShippingAddress, patchSettingsRequest,
    } = this.props;
    const {
      name, addressLineOne, city, country,
    } = this.state;

    if (_.isEmpty(name)) {
      Alert.alert( I18n.t('screens.editShippingAddress.name_required'));
      return;
    } else if (_.isEmpty(addressLineOne)) {
      Alert.alert( I18n.t('screens.editShippingAddress.address_required'));
      return;
    } else if (_.isEmpty(city)) {
      Alert.alert( I18n.t('screens.editShippingAddress.city_required'));
      return;
    } else if (_.isEmpty(country)) {
      Alert.alert( I18n.t('screens.editShippingAddress.country_required'));
      return;
    }

    const shippingIndex = navigation.getParam('shippingIndex');
    if (shippingIndex === -1) {
      setShippingAddress(0);
      eventTracker.trackEvent('Checkout-AddedAddress');

      patchSettingsRequest({ shippingAddresses: [this.state, ...shippingAddresses] });
    } else {
      const newArray = [...shippingAddresses];
      newArray[shippingIndex] = this.state;
      patchSettingsRequest({ shippingAddresses: newArray });
    }
    navigation.dispatch(NavigationActions.back());
  };

  handleCountryChange = (country) => {
    this.setState({ country: country.toUpperCase() });
  };

  scrollToInput(reactNode) { this.scroll.props.scrollToFocusedInput(reactNode); }

  goBack = () => { this.props.navigation.goBack(); };

  render() {
    const {
      name,
      postalCode,
      state,
      addressLineOne,
      addressLineTwo,
      addressNotes,
      city,
      company,
      country,
    } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={this.goBack}
          title= {I18n.t('screens.editShippingAddress.new_address')}
          right={<LinkText text= {I18n.t('screens.editShippingAddress.done')} color={linkTextColor} />}
          onRight={this.submitData}
        />
        <KeyboardAwareScrollView innerRef={this.setRef('scroll')}>
          <InputGroup title= {I18n.t('screens.editShippingAddress.your_address')}>
            <TextInput
              value={name}
              title= {I18n.t('screens.editShippingAddress.name')}
              required
              onChangeText={this.onChange('name')}
              onSubmit={this.setFocus('companyInput')}
              onFocus={this.findNodePos}
              ref={this.setRef('nameInput')}
            />
            <TextInput
              value={company}
              title= {I18n.t('screens.editShippingAddress.company')}
              onChangeText={this.onChange('company')}
              onFocus={this.findNodePos}
              onSubmit={this.setFocus('addressInput')}
              ref={this.setRef('companyInput')}
            />
            <TextInput
              value={addressLineOne}
              title= {I18n.t('screens.editShippingAddress.address')}
              required
              onChangeText={this.onChange('addressLineOne')}
              onSubmit={this.setFocus('address2Input')}
              onFocus={this.findNodePos}
              ref={this.setRef('addressInput')}
            />
            <TextInput
              value={addressLineTwo}
              title= {I18n.t('screens.editShippingAddress.address2')}
              onChangeText={this.onChange('addressLineTwo')}
              onSubmit={this.setFocus('cityInput')}
              onFocus={this.findNodePos}
              ref={this.setRef('address2Input')}
            />
            <TextInput
              value={city}
              title= {I18n.t('screens.editShippingAddress.city')}
              required
              onChangeText={this.onChange('city')}
              onSubmit={this.setFocus('stateInput')}
              onFocus={this.findNodePos}
              ref={this.setRef('cityInput')}
            />
            <TextInput
              value={state}
              title= {I18n.t('screens.editShippingAddress.state')}
              onChangeText={this.onChange('state')}
              onFocus={this.findNodePos}
              ref={this.setRef('stateInput')}
              onSubmit={this.setFocus('postalCodeInput')}
            />
            <TextInput
              value={postalCode}
              title= {I18n.t('screens.editShippingAddress.postal_code')}
              onChangeText={this.onChange('postalCode')}
              onFocus={this.findNodePos}
              ref={this.setRef('postalCodeInput')}
            />
            <RadioModalFilter
              title= {I18n.t('screens.editShippingAddress.country')}
              required
              secondary
              selected={_.isEmpty(country) ? '' : country.toLowerCase()}
              onChange={this.handleCountryChange}
              options={countries}
            />
            <TextArea
              noBorder
              value={addressNotes}
              title= {I18n.t('screens.editShippingAddress.delivery_notes')}
              onChangeText={this.onChange('addressNotes')}
              onFocus={this.findNodePos}
            />
          </InputGroup>
        </KeyboardAwareScrollView>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  shippingAddresses: state.settings.shippingAddresses,
  patchSettingsError: state.settings.patchSettingsError,
});

const mapDispatchToProps = {
  patchSettingsRequest,
  setShippingAddress,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(EditShippingAddress);
