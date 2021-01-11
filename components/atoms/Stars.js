import React from 'react';
import { View } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { starRatingColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  starContainer: {
    flexDirection: 'row',
  },
};

export default ({ score }) => {
  const nodes = [];
  for (let i = 1; i <= 5; i += 1) {
    if (i <= Math.floor(score)) {
      nodes.push(<Ionicons key={`star_${i}`} name="md-star" size={16} color={starRatingColor} />);
    } else {
      nodes.push(<Ionicons key={`star_${i}`} name="md-star" size={16} color={starRatingColor} />);
    }
  }
  return (
    <View style={styles.starContainer}>
      {nodes}
    </View>
  );
};
