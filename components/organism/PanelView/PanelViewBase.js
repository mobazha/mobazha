import React from 'react';
import { Animated, Dimensions, View, TouchableWithoutFeedback, Text, Platform } from 'react-native';

import { primaryTextColor, foregroundColor, borderColor, brandColor } from '../../commonColors';

import {I18n} from '../../../langs/I18n';

const { width: WINDOW_WIDTH, height: WINDOW_HEIGHT } = Dimensions.get('window');

const WRAPPER_HEIGHT = Platform.OS === 'ios' && WINDOW_HEIGHT < 800 ? 224 : 234;
const BOTTOM_OFFSET = Platform.OS === 'ios' ? WRAPPER_HEIGHT : WRAPPER_HEIGHT + 24;

const styles = {
  background: {
    position: 'absolute',
    width: WINDOW_WIDTH,
    height: WINDOW_HEIGHT,
    zIndex: 99,
  },
  gridWrapper: {
    width: WINDOW_WIDTH,
    height: WRAPPER_HEIGHT,
    borderTopLeftRadius: 8,
    borderTopRightRadius: 8,
    paddingTop: 26,
    backgroundColor: foregroundColor,
  },
  menuTitle: {
    color: primaryTextColor,
    fontSize: 15,
    fontWeight: 'bold',
    textAlign: 'center',
  },
  iconGrid: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginVertical: 25,
  },
  buttonWrapper: {
    borderTopWidth: 1,
    borderColor,
    paddingVertical: 16,
    alignItems: 'center',
    justifyContent: 'center',
  },
  buttonText: {
    fontSize: 15,
    color: brandColor,
    fontWeight: 'bold',
    textAlign: 'center',
  },
};

class PanelViewBase extends React.Component {
  componentDidMount() {
    this.handleHideGrid();
  }

  componentDidUpdate(prevProps) {
    const { isShowingPanel } = this.props;
    if (isShowingPanel && !prevProps.isShowingPanel) {
      this.handleShowGrid();
    }
    if (!isShowingPanel && prevProps.isShowingPanel) {
      this.handleHideGrid();
    }
  }

  fadeAction = new Animated.Value(0);
  slideAction = new Animated.Value(0);

  handleShowGrid = () => {
    Animated.timing(this.fadeAction, {
      toValue: 1,
      duration: 50,
    }).start(() => {
      Animated.timing(this.slideAction, {
        toValue: 1,
        duration: 100,
      }).start();
    });
  }

  handleHideGrid = () => {
    Animated.timing(this.slideAction, {
      toValue: 0,
      duration: 100,
    }).start(() => {
      Animated.timing(this.fadeAction, {
        toValue: 0,
        duration: 50,
      }).start(() => {
        this.props.hidePanel();
      });
    });
  }

  render() {
    const { isShowingPanel, title, children } = this.props;
    if (!isShowingPanel) {
      return null;
    }
    return (
      <TouchableWithoutFeedback onPress={this.handleHideGrid}>
        <Animated.View
          style={[
            styles.background,
            {
              backgroundColor: this.fadeAction.interpolate({
                inputRange: [0, 1],
                outputRange: ['rgba(0, 0, 0, 0)', 'rgba(0, 0, 0, 0.4)'],
              }),
            },
          ]}
        >
          <Animated.View
            style={[
              styles.gridWrapper,
              {
                top: this.slideAction.interpolate({
                  inputRange: [0, 1],
                  outputRange: [WINDOW_HEIGHT, WINDOW_HEIGHT - BOTTOM_OFFSET],
                }),
              },
            ]}
          >
            <Text style={styles.menuTitle}>{title}</Text>
            <View style={styles.iconGrid}>
              {children}
            </View>
            <TouchableWithoutFeedback onPress={this.handleHideGrid}>
              <View style={styles.buttonWrapper}>
                <Text style={styles.buttonText}>{I18n.t('components.organism.PanelView.PanelViewBase.cancel')}</Text>
              </View>
            </TouchableWithoutFeedback>
          </Animated.View>
        </Animated.View>
      </TouchableWithoutFeedback>
    );
  }
}

export default PanelViewBase;
