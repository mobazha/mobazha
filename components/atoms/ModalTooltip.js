import React from 'react';
import { View, Animated, Text } from 'react-native';

import { navHeightStyle } from '../../utils/navbar';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    position: 'absolute',
    marginTop: navHeightStyle.height + 16,
    justifyContent: 'center',
    alignSelf: 'center',
  },
  tooltip: {
    width: 'auto',
    height: 26,
    paddingHorizontal: 16,
    paddingVertical: 6,
    backgroundColor: 'rgba(108, 108, 108, 0.5)',
    borderRadius: 13,
  },
  text: {
    fontSize: 11,
    color: 'white',
  },
};

export default class ModalTooltip extends React.Component {
  aniVal = new Animated.Value(0);

  show() {
    Animated.timing(
      this.aniVal,
      {
        toValue: 1,
        duration: 500,
      },
    ).start(() => {
      Animated.timing(
        this.aniVal,
        {
          toValue: 0,
          duration: 500,
          delay: 1000,
        },
      ).start();
    });
  }

  render() {
    const { text } = this.props;
    return (
      <View style={styles.wrapper}>
        <Animated.View
          style={[
            styles.tooltip,
            {
              opacity: this.aniVal,
            },
          ]}
        >
          <Text style={styles.text}>
            {text}
          </Text>
        </Animated.View>
      </View>
    );
  }
}
