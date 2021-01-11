import React, { PureComponent } from 'react';
import { Text } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { isEmpty } from 'lodash';

import { formLabelColor, locationPinColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  location: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
    paddingTop: 2.5,
    paddingBottom: 10,
    maxWidth: 200,
  },
  secondary: {
    paddingTop: 6,
    paddingBottom: 6,
  },
  emptyLocation: {
    fontStyle: 'italic',
  },
};

class LocationPin extends PureComponent {
  render() {
    const { location, style, secondary } = this.props;

    return (
      <Text
        numberOfLines={1}
        style={[
          styles.location,
          isEmpty(location) ? styles.emptyLocation : {},
          secondary ? styles.secondary : {},
          style,
        ]}
      >
        <Ionicons name="md-pin" size={14} color={locationPinColor} />
        &nbsp;
        {location || I18n.t('components.atoms.LocationPin.unknown')}
      </Text>
    );
  }
}

export default LocationPin;
