import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Alert, View, Text, ScrollView, TouchableWithoutFeedback } from 'react-native';
import { isEmpty, isEqual } from 'lodash';
import { NavigationActions } from 'react-navigation';
import Ionicons from 'react-native-vector-icons/Ionicons';
import Icons from 'react-native-vector-icons/MaterialIcons';

import Header from '../components/molecules/Header';
import LinkText from '../components/atoms/LinkText';
import InputGroup from '../components/atoms/InputGroup';
import RadioGroup from '../components/atoms/RadioGroup';
import { primaryTextColor, secondaryTextColor, linkTextColor, brandColor } from '../components/commonColors';
import { screenWrapper } from '../utils/styles';
import { parseShippingOption, parseShippingService, initShippingOptionWithMinShippingPrice } from '../utils/stockManage';

import { patchSettingsRequest } from '../reducers/settings';
import { setShippingAddress, setShippingOption, clearShippingOption } from '../reducers/appstate';

import { priceStyle } from '../components/commonStyles';
import NavBackButton from '../components/atoms/NavBackButton';
import { convertorsMap } from '../selectors/currency';
import { parseCountryName } from '../utils/string';
import OBActionSheet from '../components/organism/ActionSheet';

import {I18n} from '../langs/I18n';

const styles = {
  addrName: {
    lineHeight: 15,
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  addrLine: {
    fontSize: 15,
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  optionWrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    flex: 1,
  },
  optionDeliveryWrapper: {
    flexDirection: 'column',
    flex: 1,
  },
  optionLabel: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    marginRight: 4,
  },
  estDelivery: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: secondaryTextColor,
  },
  moreIcon: {
    marginTop: -3,
    width: 32,
    height: 32,
    alignItems: 'flex-end',
    justifyContent: 'flex-start',
  },
  newAddress: {
    fontSize: 15,
    fontWeight: 'bold',
    color: brandColor,
    marginBottom: 20,
  },
  emptyWrapper: {
    flex: 1,
    backgroundColor: '#ffffff',
    flexDirection: 'column',
    alignItems: 'center',
    paddingTop: 50,
  },
  emptyIcon: {
    marginBottom: 10,
  },
  emptyText: {
    color: '#8a8a8f',
    fontSize: 15,
    width: 294,
    textAlign: 'center',
  },
  noShipping: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#8a8a8f',
    marginVertical: 13,
  },
};

class ShippingAddress extends PureComponent {
  static getDerivedStateFromProps(props) {
    const {
      navigation, shippingAddress, shippingOption, settings: { shippingAddresses },
    } = props;
    const shippingOptions = navigation.getParam('shippingOptions');

    let selected;
    let country;
    if (isEmpty(shippingAddresses)) {
      selected = -1;
      country = '';
    } else {
      selected = shippingAddress;
      country = shippingAddresses[selected].country;
    }

    return {
      shippingAddresses,
      selected,
      shippingOptions: selected === -1 ? (
        []
      ) : (
        shippingOptions && shippingOptions.filter(op => op.regions.includes('ALL') || op.regions.includes(country))
      ),
      shippingOption,
    };
  }

  componentDidMount() {
    this.initShippingOption = initShippingOptionWithMinShippingPrice.bind(this);
  }

  componentDidUpdate(prevProps, prevState) {
    const { routeName } = this.props.navigation.state;
    if (routeName !== 'CheckoutShippingAddress') {
      return;
    }

    const { selected, shippingAddresses } = this.state;
    const { selected: prevSelected, shippingAddresses: prevShippingAddresses } = prevState;
    if (selected === -1) {
      return;
    }
    if (
      prevSelected === -1
      || !isEqual(shippingAddresses[selected], prevShippingAddresses[prevSelected])
    ) {
      this.initShippingOption();
    }
  }

  onNewAddress = () => {
    this.props.navigation.navigate('EditShippingAddress', {
      shippingIndex: -1,
    });
  };

  setActionSheet = (component) => {
    this.actionSheet = component;
  };

  handlePressMore = actionSheetIndex => () => {
    this.actionSheetIndex = actionSheetIndex;
    this.actionSheet.show();
  };

  handleDelete = () => {
    const { patchSettingsRequest } = this.props;
    const { shippingAddresses, selected } = this.state;
    if (this.actionSheetIndex === selected) {
      this.handleAddressIndexChange(0);
    } else if (this.actionSheetIndex < selected) {
      this.handleAddressIndexChange(selected - 1);
    }

    const newArray = [...shippingAddresses];
    newArray.splice(this.actionSheetIndex, 1);
    patchSettingsRequest({ shippingAddresses: newArray });
  };

  handleChange = (index) => {
    switch (index) {
      case 0: {
        this.props.navigation.navigate('EditShippingAddress', {
          shippingIndex: this.actionSheetIndex,
        });
        break;
      }
      case 1: {
        Alert.alert(I18n.t('screens.shippingAddress.are_you_sure'), I18n.t('screens.shippingAddress.remove_address'), [
          { text:I18n.t('screens.shippingAddress.cancel')},
          { text:I18n.t('screens.shippingAddress.ok'), onPress: this.handleDelete },
        ]);
        break;
      }
      default:
        break;
    }
  };

  toNextScreen() {
    this.props.navigation.dispatch(NavigationActions.back());
  }

  handleAddressIndexChange = (selected) => {
    this.props.setShippingAddress(selected);
  };

  renderShippingAddress = (addr, index) => (
    <View style={styles.optionWrapper}>
      <View>
        {!isEmpty(addr.name) && (
          <Text style={styles.addrName}>{addr.name}</Text>
        )}
        {!isEmpty(addr.addressLineOne) && (
          <Text style={styles.addrLine}>{addr.addressLineOne}</Text>
        )}
        {!isEmpty(addr.addressLineTwo) && (
          <Text style={styles.addrLine}>{addr.addressLineTwo}</Text>
        )}
        {!(isEmpty(addr.city) && isEmpty(addr.state) && isEmpty(addr.postalCode)) && (
          <Text style={styles.addrLine}>
            {`${addr.city}`}
            {addr.state ? `, ${addr.state}` : ''}
            {addr.postalCode ? ` ${addr.postalCode}` : ''}
          </Text>
        )}
        {!isEmpty(addr.country) && (
          <Text style={styles.addrLine}>{parseCountryName(addr.country)}</Text>
        )}
      </View>
      <TouchableWithoutFeedback onPress={this.handlePressMore(index)}>
        <View style={styles.moreIcon}>
          <Ionicons name="md-more" size={24} color={primaryTextColor} />
        </View>
      </TouchableWithoutFeedback>
    </View>
  );

  renderShippingOption = (option) => {
    const { localLabelFromBCH, navigation } = this.props;
    const currency = navigation.getParam('currency');
    return (
      <View style={styles.optionWrapper}>
        <View style={styles.optionDeliveryWrapper}>
          <Text style={styles.optionLabel}>{option.service}</Text>
          <Text style={styles.estDelivery}>{option.estimatedDelivery}</Text>
        </View>
        <Text style={priceStyle}>
          {option.bigPrice === 0 ? I18n.t('screens.shippingAddress.free') : `${localLabelFromBCH(option.bigPrice, currency)}`}
        </Text>
      </View>
    );
  };

  renderShippingOptions = () => {
    const { shippingOptions, shippingOption, selected } = this.state;

    if (!shippingOptions || shippingOptions.length === 0) {
      return selected === -1 ? null : this.renderEmptyState();
    }

    return shippingOptions.map(op => (
      <InputGroup title={op.name}>
        <RadioGroup
          alignTop
          options={parseShippingOption(op)}
          selected={
            op.name === shippingOption.name &&
            op.services.findIndex(s => s.name === shippingOption.service)
          }
          onChange={idx => this.props.setShippingOption(parseShippingService(op, idx))}
          renderItem={this.renderShippingOption}
        />
      </InputGroup>
    ));
  };

  renderEmptyState = () => (
    <View style={styles.emptyWrapper}>
      <Icons style={styles.emptyIcon} name="local-shipping" size={50} color="#8a8a8f" />
      <Text style={styles.emptyText}>
      {I18n.t('screens.shippingAddress.cannot_ship')}
      </Text>
    </View>
  );

  render() {
    const { routeName } = this.props.navigation.state;
    const {
      shippingAddresses, selected, shippingOptions,
    } = this.state;
    return (
      <View style={screenWrapper.wrapper}>
        <Header
          left={<NavBackButton />}
          onLeft={() => {
            this.props.navigation.dispatch(NavigationActions.back());
          }}
          title= {I18n.t('screens.shippingAddress.Shipping')}
          right={<LinkText text={I18n.t('screens.shippingAddress.done')} color={linkTextColor} />}
          onRight={() => {
            this.toNextScreen();
          }}
        />
        <ScrollView>
          <Text>{isEmpty(shippingOptions)}</Text>
          <InputGroup title= {I18n.t('screens.shippingAddress.ships_to')}>
            {isEmpty(shippingAddresses) ? (
              <Text style={styles.noShipping}>{I18n.t('screens.shippingAddress.no_address')}</Text>
            ) : (
              <RadioGroup
                alignTop
                options={shippingAddresses}
                selected={selected}
                onChange={this.handleAddressIndexChange}
                renderItem={this.renderShippingAddress}
                showSeparator
              />
            )}
            <TouchableWithoutFeedback onPress={this.onNewAddress}>
              <View>
                <Text style={styles.newAddress}>{I18n.t('screens.shippingAddress.add_address')} </Text>
              </View>
            </TouchableWithoutFeedback>
          </InputGroup>
          {routeName === 'CheckoutShippingAddress' && this.renderShippingOptions()}
        </ScrollView>
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={['Edit', 'Delete', 'Cancel']}
          cancelButtonIndex={2}
          destructiveButtonIndex={1}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  settings: state.settings,
  shippingAddress: state.appstate.shippingAddress,
  shippingOption: state.appstate.shippingOption,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  patchSettingsRequest,
  setShippingAddress,
  setShippingOption,
  clearShippingOption,
};

export default connect(
  mapStateToProps,
  mapDispatchToProps,
)(ShippingAddress);
