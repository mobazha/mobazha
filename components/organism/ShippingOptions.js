import React from 'react';
import { Text } from 'react-native';
import { connect } from 'react-redux';
import _ from 'lodash';

import InputGroup from '../atoms/InputGroup';
import OptionGroup from '../atoms/OptionGroup';
import FormLabelText from '../atoms/FormLabelText';
import { primaryTextColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const style = {
  fontSize: 15,
  fontWeight: 'normal',
  fontStyle: 'normal',
  letterSpacing: 0,
  textAlign: 'left',
  color: primaryTextColor,
  paddingVertical: 20,
};

class ShippingOptions extends React.PureComponent {
  onPress = () => {
    this.props.onPress('ShippingOptions');
  };

  renderLabel = () => {
    const { shippingOptions } = this.props;
    switch (shippingOptions.length) {
      case 0:
        return <FormLabelText text={I18n.t('components.organism.ShippingOptions.add_option')} />;
      case 1:
        return <Text style={style}>{shippingOptions[0].name}</Text>;
      default:
        return <Text style={style}>{I18n.t('components.organism.ShippingOptions.options_count', {count: shippingOptions.length})}</Text>;
    }
  }

  render() {
    return (
      <InputGroup title={I18n.t('components.organism.ShippingOptions.shipping')} required showPencil onPress={this.onPress}>
        <OptionGroup noBorder noArrow>
          {this.renderLabel()}
        </OptionGroup>
      </InputGroup>
    );
  }
}

const mapStateToProps = state => ({
  shippingOptions: state.createListing.shippingOptions,
  worldWideShipping: state.createListing.worldWideShipping,
});

export default connect(mapStateToProps)(ShippingOptions);
