import React from 'react';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { borderColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 16,
  },
  borderBottom: {
    borderBottomWidth: 1,
    borderColor,
  },
  title: {
    fontSize: 15,
    color: '#777777',
  },
  starWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  star: {
    marginHorizontal: 3,
  },
};

const renderStar = (value, onPress) => {
  const nodes = [];
  for (let i = 0; i < 5; i += 1) {
    nodes.push((
      <TouchableWithoutFeedback onPress={() => { onPress(i + 1); }} key={`star_${i}`}>
        <Ionicons
          name="md-star"
          style={styles.star}
          size={25}
          color={i < value ? '#f9d553' : '#c8c7cc'}
        />
      </TouchableWithoutFeedback>
    ));
  }
  return nodes;
};

export default ({
  title,
  value,
  onPress,
  noBorder,
}) => (
  <View style={[styles.wrapper, noBorder ? {} : styles.borderBottom]}>
    <Text style={styles.title}>
      {title}
    </Text>
    <View style={styles.starWrapper}>
      {
        renderStar(value, onPress)
      }
    </View>
  </View>
);
