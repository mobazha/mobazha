import React, { PureComponent } from 'react';
import { Platform, View, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { backgroundColor } from '../commonColors';
import NavBackButton from '../atoms/NavBackButton';
import StatusBarWrapper from '../../status-bar';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    zIndex: 9999,
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
  },
  contentWrapper: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    width: '100%',
    paddingTop: 5,
    paddingLeft: 6,
    paddingRight: 6,
  },
  btnWrapper: {
    flexDirection: 'row',
    padding: 4,
  },
  btn: {
    backgroundColor: 'rgba(108, 108, 108, 0.5)',
    width: 40,
    height: 40,
    borderRadius: 20,
    justifyContent: 'center',
    alignItems: 'center',
  },
};

export default class EditProfileHeader extends PureComponent {
  render() {
    const { onBack } = this.props;
    return (
      <View style={styles.wrapper}>
        <StatusBarWrapper
          backgroundColor="white"
          barStyle="dark-content"
        />
        <View style={styles.contentWrapper}>
          <View style={styles.btnWrapper}>
            <TouchableWithoutFeedback onPress={onBack}>
              <View style={styles.btn}>
                <NavBackButton white />
              </View>
            </TouchableWithoutFeedback>
          </View>
        </View>
      </View>
    );
  }
}
