import React from 'react';
import { TouchableWithoutFeedback, Animated, View } from 'react-native';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    height: 32,
    width: 46,
    paddingLeft: 2,
  },
  slot: {
    marginRight: 1,
    marginTop: 3,
    width: 34,
    height: 14,
    borderRadius: 7,
  },
  switcher: {
    position: 'absolute',
    top: 0,
    width: 20,
    height: 20,
    borderRadius: 10,
    shadowColor: '#000000',
    shadowOpacity: 0.4,
    shadowOffset: { width: 0, height: 2 },
    shadowRadius: 2,
  },
};

export default class Switcher extends React.Component {
  constructor(props) {
    super(props);
    this.aniVal = new Animated.Value(props.value ? 1 : 0);
    this.state = {
      value: props.value,
    };
  }

  componentDidUpdate(prevProps) {
    const { value } = this.props;
    if (prevProps.value && !value) {
      Animated.timing(this.aniVal, {
        toValue: 0,
        duration: 0,
      }).start();
    } else if (!prevProps.value && value) {
      Animated.timing(this.aniVal, {
        toValue: 1,
        duration: 0,
      }).start();
    }
  }

  onChange = () => {
    const { value } = this.state;
    if (value) {
      Animated.timing(this.aniVal, {
        toValue: 0,
        duration: 250,
      }).start(() => {
        this.setState({
          value: !value,
        });
        this.props.onChange(!value);
      });
    } else {
      Animated.timing(this.aniVal, {
        toValue: 1,
        duration: 250,
      }).start(() => {
        this.setState({
          value: !value,
        });
        this.props.onChange(!value);
      });
    }
  };

  render() {
    return (
      <TouchableWithoutFeedback onPress={this.onChange}>
        <View style={styles.wrapper}>
          <Animated.View
            style={[
              styles.slot,
              {
                backgroundColor: this.aniVal.interpolate({
                  inputRange: [0, 1],
                  outputRange: ['#221f1f42', '#00bf65'],
                }),
              },
            ]}
          />
          <Animated.View
            elevation={4}
            style={[
              styles.switcher,
              {
                backgroundColor: this.aniVal.interpolate({
                  inputRange: [0, 1],
                  outputRange: ['#f1f1f1', '#00bf65'],
                }),
                left: this.aniVal.interpolate({
                  inputRange: [0, 1],
                  outputRange: [2, 16],
                }),
              },
            ]}
          />
        </View>
      </TouchableWithoutFeedback>
    );
  }
}
