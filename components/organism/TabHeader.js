import React, { PureComponent } from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import * as _ from 'lodash';

import { foregroundColor, brandColor } from '../commonColors';
import { navHeightStyle } from '../../utils/navbar';
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
    backgroundColor: brandColor,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  title: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: foregroundColor,
  },
  left: {
    width: 70,
    alignItems: 'flex-start',
  },
  center: {
    flex: 1,
    justifyContent: 'center',
  },
  right: {
    width: 70,
    alignItems: 'flex-end',
  },
};

class TabHeader extends PureComponent {
  render() {
    const {
      title, left, onLeft, right, onRight,
    } = this.props;
    return (
      <View style={styles.wrapper}>
        <StatusBarWrapper
          backgroundColor={brandColor}
          barStyle="light-content"
        />
        <View style={styles.contentWrapper}>
          <View style={styles.left}>
            <TouchableWithoutFeedback onPress={onLeft}>
              <View>
                {left}
              </View>
            </TouchableWithoutFeedback>
          </View>
          <View style={styles.center}>
            <Text style={styles.title}>{title}</Text>
          </View>
          <View style={styles.right}>
            <TouchableWithoutFeedback onPress={onRight}>
              <View>
                {right}
              </View>
            </TouchableWithoutFeedback>
          </View>
        </View>
      </View>
    );
  }
}

export default TabHeader;
