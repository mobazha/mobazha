import React from 'react';
import { Animated, Dimensions, PanResponder, TouchableWithoutFeedback } from 'react-native';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const DEFAULT_CONTENT_WRAPPER_HEIGHT = 200;

const styles = {
  wrapper: {
    position: 'absolute',
    width: '100%',
    height: '100%',
    justifyContent: 'flex-end',
  },
  contentWrapper: {
    width: SCREEN_WIDTH,
    height: DEFAULT_CONTENT_WRAPPER_HEIGHT,
    backgroundColor: 'white',
  },
};

export default class OBSlidingPanel extends React.Component {
  constructor(props) {
    super(props);
    const { height = DEFAULT_CONTENT_WRAPPER_HEIGHT } = props;
    this.panResponder = PanResponder.create({
      onStartShouldSetPanResponder: () => true,
      onStartShouldSetPanResponderCapture: () => false,
      onMoveShouldSetPanResponder: (evt, gestureState) => {
        const { dx, dy } = gestureState;
        return dx > 10 || dx < -10 || dy > 10 || dy < -10;
      },
      onMoveShouldSetPanResponderCapture: () => true,
      onPanResponderMove: (evt, gestureState) => {
        const { dy } = gestureState;
        if (dy >= 0) {
          this.verticalAniVal.setValue(0 - dy);
        }
      },
      onPanResponderRelease: (evt, gestureState) => {
        const { dy } = gestureState;
        if (dy >= 100) {
          this.disappear();
        } else {
          this.resetVericalPosition();
        }
      },
      onShouldBlockNativeResponder: () => true,
    });
    this.verticalAniVal = new Animated.Value(-height);
  }

  state = { show: false };

  componentDidUpdate(prevProps) {
    const { height } = this.props;
    if (height !== prevProps.height) {
      this.verticalAniVal = new Animated.Value(-height);
    }
  }

  bgAniVal = new Animated.Value(0);

  verticalAniVal = new Animated.Value(-DEFAULT_CONTENT_WRAPPER_HEIGHT);

  show() {
    this.setState({ show: true }, () => { this.appear(); });
  }

  hide() {
    this.setState({ show: false }, () => { this.disappear(); });
  }

  appear = () => {
    Animated.timing(this.bgAniVal, {
      toValue: 1,
      duration: 200,
    }).start(() => {
      Animated.timing(this.verticalAniVal, {
        toValue: 0,
        duration: 200,
      }).start();
    });
  }

  disappear = () => {
    const { height = DEFAULT_CONTENT_WRAPPER_HEIGHT } = this.props;
    Animated.timing(this.verticalAniVal, {
      toValue: -height,
      duration: 200,
    }).start(() => {
      Animated.timing(this.bgAniVal, {
        toValue: 0,
        duration: 200,
      }).start(() => {
        this.setState({ show: false });
      });
    });
  }

  resetVericalPosition = () => {
    Animated.timing(this.verticalAniVal, {
      toValue: 0,
      duration: 200,
    }).start();
  }

  render() {
    const { show } = this.state;
    const { children, height = DEFAULT_CONTENT_WRAPPER_HEIGHT } = this.props;
    if (!show) {
      return false;
    }
    return (
      <TouchableWithoutFeedback onPress={this.disappear}>
        <Animated.View
          style={[
            styles.wrapper,
            {
              backgroundColor: this.bgAniVal.interpolate({
                inputRange: [0, 1],
                outputRange: ['rgba(0, 0, 0, 0)', 'rgba(0, 0, 0, 0.7)'],
              }),
            },
          ]}
        >
          <Animated.View
            style={[
              styles.contentWrapper,
              { height, marginBottom: this.verticalAniVal },
            ]}
            {...this.panResponder.panHandlers}
          >
            {children}
          </Animated.View>
        </Animated.View>
      </TouchableWithoutFeedback>
    );
  }
}
