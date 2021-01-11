import React from 'react';
import { View, Text } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { formLabelColor, starRatingColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 5,
  },
  title: {
    width: 140,
    fontSize: 15,
    color: formLabelColor,
  },
  starWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  star: {
    marginHorizontal: 3,
  },
};

export default ({ title, value }) => {
  const stars = [];
  for (let i = 0; i < 5; i += 1) {
    stars.push((
      <Ionicons
        name="md-star"
        style={styles.star}
        size={17}
        color={i < value ? starRatingColor : '#c8c7cc'}
        key={`star_${i}`}
      />
    ));
  }
  return (
    <View style={styles.wrapper}>
      <Text style={styles.title}>
        {title}
      </Text>
      <View style={styles.starWrapper}>
        { stars }
      </View>
    </View>
  );
};
