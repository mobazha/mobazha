import React from 'react';
import { View, Text, Linking, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import * as _ from 'lodash';

import { primaryTextColor, staticLabelColor, brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  fieldWrapper: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    marginRight: 12,
  },
  fieldLabel: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: staticLabelColor,
    marginRight: 3,
  },
  fieldValue: {
    flex: 1,
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  linkText: {
    textDecorationLine: 'underline',
  },
};

export default class StaticField extends React.PureComponent {
  handleOpenURL = () => {
    const { value } = this.props;
    if (_.isEmpty(value)) {
      return;
    }
    Linking.canOpenURL(value)
      .then(supported => supported && Linking.openURL(value))
      .catch(() => {});
  }

  render() {
    const {
      label, value, onCopy, isLink,
    } = this.props;
    const description = _.isEmpty(value) ? 'N/A' : value;
    return (
      <View style={styles.wrapper}>
        <View style={styles.fieldWrapper}>
          <Text style={styles.fieldLabel} numberOfLines={1}>
            {`${label} `}
          </Text>
          {isLink ? (
            <TouchableWithoutFeedback onPress={this.handleOpenURL}>
              <Text style={[styles.fieldValue, styles.linkText]} numberOfLines={1} ellipsizeMode="tail">
                {description}
              </Text>
            </TouchableWithoutFeedback>
          ) : (
            <Text style={styles.fieldValue} numberOfLines={1} ellipsizeMode="tail">
              {description}
            </Text>
          )}
        </View>
        {!_.isEmpty(value) && onCopy && (
          <TouchableWithoutFeedback onPress={onCopy} >
            <View>
              <Ionicons name="md-copy" color={brandColor} size={22} />
            </View>
          </TouchableWithoutFeedback>
        )}
      </View>
    );
  }
}
