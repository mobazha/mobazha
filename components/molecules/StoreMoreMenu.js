import React, { PureComponent } from 'react';
import { TouchableWithoutFeedback, View, Animated } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { primaryTextColor, foregroundColor } from '../commonColors';

import {I18n} from '../../langs/I18n';

const styles = {
  moreIcon: {
    width: 32,
    height: 32,
    textAlign: 'center',
    alignItems: 'center',
    justifyContent: 'center',
  },
};

class StoreMoreMenu extends PureComponent {
  state = {};

  render() {
    const {
      onMore, black, size, animColor,
    } = this.props;
    const IconComponent = animColor ? Animated.createAnimatedComponent(Ionicons) : Ionicons;
    const color = black ? primaryTextColor : foregroundColor;
    return (
      <TouchableWithoutFeedback onPress={onMore}>
        <View style={styles.moreIcon}>
          <IconComponent
            name="md-more"
            size={size || 30}
            color={!animColor && color}
            style={[{ textAlign: 'center' }, animColor && { color: animColor }]}
          />
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

export default StoreMoreMenu;
