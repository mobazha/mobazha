import React from 'react';
import { connect } from 'react-redux';
import { View, Text } from 'react-native';

import { primaryTextColor } from '../commonColors';
import { priceStyle } from '../commonStyles';

import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 14,
  },
  title: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
    flex: 1,
    marginRight: 15,
  },
};

export default connect(convertorsMap)(({
  title, price, currency, quantity, localLabelFromBCH,
}) => (
  <View style={styles.wrapper}>
    <Text style={styles.title} numberOfLines={1}>
      {title}
    </Text>
    <Text style={priceStyle}>{localLabelFromBCH(price * quantity, currency)}</Text>
  </View>
));
