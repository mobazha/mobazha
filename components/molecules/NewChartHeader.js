import React from 'react';
import { View, TouchableOpacity } from 'react-native';

import { foregroundColor } from '../commonColors';
import { navHeightStyle } from '../../utils/navbar';
import NavBackButton from '../atoms/NavBackButton';
import QRScanner from '../organism/QRScanner';
import StatusBarWrapper from '../../status-bar';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    alignSelf: 'stretch',
  },
  contentWrapper: {
    paddingLeft: 6,
    paddingRight: 6,
    ...navHeightStyle,
    backgroundColor: 'white',
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  left: {
    width: 48,
    alignItems: 'flex-start',
  },
  center: {
    flex: 1,
    justifyContent: 'center',
  },
  right: {
    width: 56,
    alignItems: 'center',
  },
};

export default ({
  onBack, searchComponent,
}) => (
  <View style={styles.wrapper}>
    <StatusBarWrapper backgroundColor={foregroundColor} barStyle="dark-content" />
    <View style={styles.contentWrapper}>
      <View style={styles.left}>
        <TouchableOpacity onPress={onBack}>
          <NavBackButton />
        </TouchableOpacity>
      </View>
      <View style={styles.center}>
        {searchComponent}
      </View>
      <View style={styles.right}>
        <QRScanner black />
      </View>
    </View>
  </View>
);
