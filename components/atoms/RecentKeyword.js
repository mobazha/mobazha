import React from 'react';
import { TouchableWithoutFeedback, Text, View, Platform } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { primaryTextColor, foregroundColor, formLabelColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingLeft: 15,
    paddingRight: 12,
    paddingVertical: 10,
  },
  text: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    justifyContent: 'center',
    lineHeight: 30,
  },
  iconWrapper: {
    height: 30,
    width: 30,
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
  },
};

const CloseIcon = () => {
  const iconName = Platform.OS === 'ios' ? 'ios-close' : 'md-close';
  const iconSize = Platform.OS === 'ios' ? 24 : 18;
  return (
    <Ionicons name={iconName} size={iconSize} color={formLabelColor} />
  );
};

export default ({
  keyword, onSelect, onRemove, isLast, disableRemove,
}) => (
  <TouchableWithoutFeedback
    onPress={() => onSelect(keyword)}
  >
    <View style={[styles.wrapper, isLast ? styles.last : null]}>
      <Text style={styles.text}>
        {keyword}
      </Text>
      {!disableRemove && (
        <TouchableWithoutFeedback onPress={() => onRemove(keyword)}>
          <View style={styles.iconWrapper}>
            <CloseIcon />
          </View>
        </TouchableWithoutFeedback>
      )}
    </View>
  </TouchableWithoutFeedback>
);
