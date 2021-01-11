import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { borderColor, brandColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    paddingVertical: 20,
    alignItems: 'center',
  },
  checkWrapper: {
    width: 18,
    height: 18,
    borderWidth: 2,
    borderRadius: 2,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 6,
  },
  title: {
    fontSize: 15,
    fontWeight: 'normal',
    color: '#000',
  },
};

export default ({ checked, title, onPress }) => (
  <TouchableWithoutFeedback onPress={onPress}>
    <View style={styles.wrapper}>
      <View style={[styles.checkWrapper, { backgroundColor: checked ? brandColor : 'white', borderColor: checked ? brandColor : borderColor }]}>
        {checked && <Ionicons name="md-checkmark" size={16} color="white" />}
      </View>
      <Text style={styles.title}>{title}</Text>
    </View>
  </TouchableWithoutFeedback>
);
