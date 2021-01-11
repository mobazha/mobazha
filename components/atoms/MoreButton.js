import React from 'react';
import { TouchableWithoutFeedback, Text, View } from 'react-native';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    marginBottom: 20,
  },
  text: {
    fontSize: 14,
    fontWeight: 'bold',
    color: '#00bf65',
  },
};

export default ({ onPress, title }) => (
  <TouchableWithoutFeedback onPress={onPress}>
    <View style={styles.wrapper}>
      <Text style={styles.text}>+ {title || I18n.t('components.atoms.MoreButton.more')}</Text>
    </View>
  </TouchableWithoutFeedback>
);
