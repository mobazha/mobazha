import React from 'react';
import { View, Text, TouchableWithoutFeedback, Image } from 'react-native';

import InputGroup from '../atoms/InputGroup';
import { foregroundColor, primaryTextColor, staticLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const coinBase = require('../../assets/icons/crypto/coinbase.png');

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    paddingHorizontal: 16,
    marginVertical: 12,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  itemContent: {
  },
  image: {
    width: 34,
    height: 34,
  },
  text: {
    fontSize: 14,
    fontWeight: 'bold',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    paddingLeft: 8,
  },
  subtitle: {
    fontSize: 12,
    letterSpacing: 0,
    marginTop: 2,
    color: staticLabelColor,
    textAlign: 'left',
    paddingLeft: 8,
  },
};

export default NeedCoin = ({
  toCoinbase,
}) => (
  <InputGroup
    title="Services"
    noPadding
    noBorder
  >
    <View style={styles.wrapper}>
      <View style={styles.row}>
        <Image style={styles.image} source={coinBase} />
        <TouchableWithoutFeedback delayPressIn={200} >
          <View style={styles.itemContent}>
            <Text style={styles.text}>{I18n.t('components.templates.NeedCoin.coinbase')}</Text>
            <Text style={styles.subtitle}>{I18n.t('components.templates.NeedCoin.cryptocurrency_exchange')}</Text>
          </View>
        </TouchableWithoutFeedback>
      </View>
    </View>
  </InputGroup>
);
