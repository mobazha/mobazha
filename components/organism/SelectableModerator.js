import React from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback } from 'react-native';
import LinearGradient from 'react-native-linear-gradient';
import * as striptags from 'striptags';
import Icon from 'react-native-vector-icons/FontAwesome';
import he from 'he';
import decode from 'unescape';

import Avatar from '../atoms/AvatarImage';
import { primaryTextColor, linkTextColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';
import { getCurrencySymbol } from '../../utils/currency';
import { eatSpaces } from '../../utils/string';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: 'rgb(233, 131, 30)',
    borderRadius: 2,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#e9831e',
    paddingHorizontal: 14,
    paddingTop: 12,
    paddingBottom: 15,
    marginTop: -1,
  },
  profile: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 12,
  },
  profileInfo: {
    flex: 1,
    flexDirection: 'column',
    marginLeft: 5,
  },
  description: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 19,
    letterSpacing: 0,
    color: primaryTextColor,
  },
  toDetail: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 19,
    letterSpacing: 0,
    color: linkTextColor,
  },
  briefContainer: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  location: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    color: primaryTextColor,
  },
  fee: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    lineHeight: 17,
    letterSpacing: 0,
    color: primaryTextColor,
    paddingLeft: 15,
  },
  checkBox: {
    width: 19,
    height: 19,
    backgroundColor: '#ffffff',
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 8,
  },
};

class SelectableModerator extends React.PureComponent {
  getFeeText() {
    const {
      profile: {
        moderatorInfo: {
          fee: { feeType, percentage, fixedFee },
        },
      },
      convertCurrency,
    } = this.props;
    switch (feeType) {
      case 'FIXED': {
        const symbol = getCurrencySymbol(fixedFee.currencyCode);
        return `${symbol} ${convertCurrency(fixedFee.amount, fixedFee.currencyCode)} per case*`;
      }
      case 'PERCENTAGE':
        return `${percentage}% per case*`;
      default: {
        const symbol = getCurrencySymbol(fixedFee.currencyCode);
        const fixedText = `${symbol}${convertCurrency(fixedFee.amount, fixedFee.currencyCode)}`;
        return `${fixedText} + ${percentage}%  per case*`;
      }
    }
  }
  render() {
    const {
      profile: {
        avatarHashes: { tiny: avatarHash },
        name,
        location,
        moderatorInfo: { description },
      },
      checked,
      onPress,
    } = this.props;
    return (
      <LinearGradient
        start={{ x: 0, y: 0 }}
        end={{ x: 0, y: 1 }}
        colors={['rgb(255, 250, 246)', 'rgb(255, 244, 233)']}
        style={styles.wrapper}
      >
        <View style={styles.content}>
          <View style={styles.profile}>
            <TouchableWithoutFeedback onPress={onPress}>
              <View style={styles.checkBox}>
                {checked && <Icon name="check" size={14} color="#00BF65" />}
              </View>
            </TouchableWithoutFeedback>
            <View style={styles.imgWrapper}>
              <Avatar thumbnail={avatarHash} style={{ width: 30 }} />
            </View>
            <View style={styles.profileInfo}>
              <Text style={styles.name}>{decode(name)}</Text>
              <View style={styles.briefContainer}>
                <Text style={styles.location}>📍 {decode(location)}</Text>
                <Text style={styles.fee}>{this.getFeeText()}</Text>
              </View>
            </View>
          </View>
          <View style={styles.wholeDescription}>
            <Text style={styles.description} numberOfLines={3} ellipsizeMode="tail">
              {eatSpaces(he.decode(striptags.default(description)))}
            </Text>
            <Text style={styles.toDetail}>{I18n.t('components.organism.SelectableModerator.view_details')}</Text>
          </View>
        </View>
      </LinearGradient>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(SelectableModerator);
