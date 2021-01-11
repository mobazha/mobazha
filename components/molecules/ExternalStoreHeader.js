import React, { PureComponent } from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import * as _ from 'lodash';
import decode from 'unescape';

import { primaryTextColor } from '../commonColors';
import { navHeightStyle } from '../../utils/navbar';
import NavBackButton from '../atoms/NavBackButton';
import StoreMoreMenu from '../molecules/StoreMoreMenu';
import StatusBarWrapper from '../../status-bar';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    alignSelf: 'stretch',
    width: '100%',
  },
  contentWrapper: {
    paddingLeft: 6,
    paddingRight: 6,
    ...navHeightStyle,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    width: '100%',
  },
  leftBtn: {
    width: 32,
    height: 32,
    justifyContent: 'center',
    alignItems: 'flex-start',
  },
  title: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'bold',
    textAlign: 'center',
    color: primaryTextColor,
  },
  rightSpacer: {
    width: 32,
    height: 32,
  },
};

class ExternalStoreHeader extends PureComponent {
  render() {
    const { profile, isBlocked, onMore } = this.props;
    return (
      <View style={styles.wrapper}>
        <StatusBarWrapper
          backgroundColor="white"
          barStyle="dark-content"
        />
        <View style={styles.contentWrapper}>
          <TouchableWithoutFeedback onPress={this.props.onBack}>
            <View style={styles.leftBtn}>
              <NavBackButton />
            </View>
          </TouchableWithoutFeedback>
          <Text style={styles.title} numberOfLines={1}>
            {decode(_.get(profile, 'name'))}
          </Text>
          {!isBlocked && onMore ? (
            <StoreMoreMenu onMore={onMore} black />
          ) : (
            <View style={styles.rightSpacer} />
          )}
        </View>
      </View>
    );
  }
}

export default ExternalStoreHeader;
