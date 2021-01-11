import React from 'react';
import { connect } from 'react-redux';
import { View, TouchableWithoutFeedback } from 'react-native';
import Feather from 'react-native-vector-icons/Feather';

import { brandColor } from '../commonColors';

import { showPanel } from '../../reducers/appstate';

import {I18n} from '../../langs/I18n';

const styles = {
  container: {
    width: '100%',
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    height: '100%',
  },
  wrapper: {
    width: 40,
    height: 40,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: brandColor,
    borderRadius: 20,
  },
  icon: {
    width: 24,
    height: 24,
    textAlign: 'center',
  },
};

class PlusButton extends React.PureComponent {
  onPress = () => {
    this.props.showPanel({ type: 'plus' });
  }

  render() {
    return (
      <TouchableWithoutFeedback onPress={this.onPress}>
        <View style={styles.container}>
          <View style={styles.wrapper}>
            <Feather style={styles.icon} name="plus" size={24} color="white" />
          </View>
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapDispatchToProps = {
  showPanel,
};

export default connect(
  null,
  mapDispatchToProps,
)(PlusButton);
