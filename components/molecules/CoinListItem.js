import React, { PureComponent } from 'react';
import { Alert, View, Text, Image, TouchableWithoutFeedback } from 'react-native';
import { connect } from 'react-redux';
import Ionicons from 'react-native-vector-icons/Ionicons';

import { staticLabelColor, brandColor, primaryTextColor } from '../commonColors';
import { COINS } from '../../utils/coins';
import { convertorsMap } from '../../selectors/currency';
import { cellStyles } from '../../utils/styles';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
  },
  contentWrapper: {
    flexDirection: 'row',
    padding: 15,
    alignItems: 'center',
  },
  leftWrapper: {
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    paddingLeft: 8,
  },
  icon: {
    width: 34,
    height: 34,
  },
  coinName: {
    fontSize: 15,
    fontWeight: 'bold',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  secondary: {
    fontSize: 13,
    letterSpacing: 0,
    marginTop: 2,
    color: staticLabelColor,
  },
  iconForward: {
    paddingRight: 15,
  },
};

class CoinListItem extends PureComponent {
  handlePress = () => {
    const { coin, onCoinSelected } = this.props;
    const { disabled } = COINS[coin];
    if (disabled) {
      Alert.alert('Coming soon!');
    } else {
      onCoinSelected(coin);
    }
  }

  render() {
    const { coin, selected } = this.props;
    const { label, icon } = COINS[coin];

    return (
      <View style={styles.wrapper}>
        <TouchableWithoutFeedback
          onPress={this.handlePress}
        >
          <View style={styles.contentWrapper}>
            <Image style={styles.icon} source={icon} resizeMode="cover" />
            <View style={styles.leftWrapper}>
              <Text style={styles.coinName}>{label}</Text>
              <Text style={styles.secondary}>{coin}</Text>
            </View>
            <View style={{ flex: 1 }} />
            {selected && (
              <Ionicons
                name="ios-checkmark"
                size={24}
                color={brandColor}
                style={styles.iconForward}
              />
            )}
          </View>
        </TouchableWithoutFeedback>
        <View style={cellStyles.separatorContainer}>
          <View style={cellStyles.separator} />
        </View>
      </View>
    );
  }
}

export default connect(convertorsMap)(CoinListItem);
