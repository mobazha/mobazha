import React from 'react';
import { View, TouchableWithoutFeedback, Text, Platform } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import { brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: 'white',
    justifyContent: 'space-between',
    marginBottom: ifIphoneX(32, 16),
    paddingTop: 12,
    paddingBottom: Platform.OS === 'ios' ? 12 : 0,
    paddingHorizontal: 15,
  },
  left: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    height: 40,
    borderWidth: 1,
    borderRadius: 2,
    borderColor: brandColor,
    width: '48.5%',
  },
  right: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    height: 40,
    borderWidth: 1,
    borderRadius: 2,
    borderColor: brandColor,
    width: '48.5%',
  },
  text: {
    fontSize: 13,
    fontWeight: 'bold',
    letterSpacing: 0,
    textAlign: 'center',
    color: brandColor,
    marginRight: 4,
    lineHeight: 40,
    height: 40,
  },
  iconStyle: {
    lineHeight: 40,
  },
};

export default ({ sendMoney, receiveMoney }) => (
  <View style={styles.wrapper}>
    <TouchableWithoutFeedback onPress={receiveMoney}>
      <View style={styles.left}>
        <Text style={styles.text}>{I18n.t('components.templates.SendReceiveMoney.Receive')}</Text>
        <Ionicons
          name="md-download"
          color={brandColor}
          size={12}
          style={styles.iconStyle}
        />
      </View>
    </TouchableWithoutFeedback>
    <TouchableWithoutFeedback onPress={sendMoney}>
      <View style={styles.right}>
        <Text style={styles.text}>{I18n.t('components.templates.SendReceiveMoney.Send')}</Text>
        <Ionicons
          name="md-send"
          color={brandColor}
          size={12}
          style={styles.iconStyle}
        />
      </View>
    </TouchableWithoutFeedback>
  </View>
);
