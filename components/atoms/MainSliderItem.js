import React from 'react';
import { View, TouchableWithoutFeedback, Image, Dimensions } from 'react-native';
import { withNavigation } from 'react-navigation';

import { handleOBDeeplinkWithNavigation } from '../../utils/navigation';

import {I18n} from '../../langs/I18n';

const { width } = Dimensions.get('window');

const IMAGE_RATIO = 328.0 / 750;

const styles = {
  wrapper: {
    margin: 8,
    width: width - 16,
    height: width * IMAGE_RATIO - 16,
  },
  image: {
    width: '100%',
    height: '100%',
    borderRadius: 5,
    overflow: 'hidden',
    borderStyle: 'solid',
    borderColor: '#E8E8E8',
    borderWidth: 1,
  },
};

class MainSliderItem extends React.PureComponent {
  handlePress = () => {
    const { link } = this.props.source;
    handleOBDeeplinkWithNavigation(link, this.props.navigation);
  }

  render() {
    const { source, onLoadEnd } = this.props;
    return (
      <TouchableWithoutFeedback onPress={this.handlePress}>
        <View style={styles.wrapper}>
          <Image style={styles.image} source={{ uri: source.image }} onLoadEnd={onLoadEnd} />
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

export default withNavigation(MainSliderItem);
