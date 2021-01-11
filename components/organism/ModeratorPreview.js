import React from 'react';
import { connect } from 'react-redux';
import { View, Text, Image } from 'react-native';
import { get } from 'lodash';
import * as striptags from 'striptags';
import he from 'he';
import decode from 'unescape';
import FontAwesome from 'react-native-vector-icons/FontAwesome';

import Avatar from '../atoms/AvatarImage';
import LocationPin from '../atoms/LocationPin';
import { primaryTextColor, linkTextColor, foregroundColor, borderColor, brandColor, formLabelColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';
import { getCurrencySymbol } from '../../utils/currency';
import { eatSpaces } from '../../utils/string';

import CheckDecagramIcon from '../../assets/icons/check-decagram.png';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    backgroundColor: foregroundColor,
    borderBottomWidth: 1,
    borderBottomColor: borderColor,
    paddingHorizontal: 22,
    paddingTop: 12,
    paddingBottom: 15,
    flexDirection: 'row',
  },
  profileInfo: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 2,
  },
  verifiedWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    marginLeft: 5,
  },
  verifiedText: {
    marginLeft: 2,
    fontSize: 13,
    fontWeight: 'bold',
    color: brandColor,
  },
  descriptionWrapper: {
    flex: 1,
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
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: formLabelColor,
    paddingTop: 6,
    paddingBottom: 6,
  },
  verifyIcon: {
    width: 15,
    height: 15,
  },
  dollarSign: {
    marginLeft: 15,
    marginRight: 3,
    color: '#8CD985',
  },
  imgWrapper: {
    marginRight: 8,
  },
  avatarImage: {
    width: 40,
  },
  name: {
    color: primaryTextColor,
    fontWeight: 'bold',
    flex: 1,
  },
};

const formatDecimal = (num) => {
  const str = num.toString();
  const splittedParts = str.split('.');
  if (splittedParts.length < 2) {
    return str;
  }

  const intPart = splittedParts[0];
  const decPart = splittedParts[1];
  if (intPart.length > 1 || intPart[0] !== '0') {
    return num.toFixed(2);
  }
  let cutPos = 0;
  for (let i = 0; i < decPart.length; i += 1) {
    if (decPart[i] !== '0') {
      cutPos = i;
      break;
    }
  }
  return `${intPart}.${decPart.slice(0, cutPos + 1)}`;
};

class ModeratorPreview extends React.PureComponent {
  getFeeText() {
    const { profile = {}, convertCurrency } = this.props;
    const { fee } = profile.moderatorInfo || {};
    if (!fee) {
      return '';
    }
    const { feeType, percentage, fixedFee } = fee;
    const { currencyCode, amountCurrency = {}, bigAmount } = fixedFee || {};
    switch (feeType) {
      case 'FIXED': {
        const symbol = getCurrencySymbol(currencyCode);
        const feeFormated = formatDecimal(convertCurrency(bigAmount, amountCurrency.code));
        return `${symbol} ${feeFormated}`;
      }
      case 'PERCENTAGE':
        return `${percentage}%`;
      default: {
        const symbol = getCurrencySymbol(currencyCode);
        const feeFormated = formatDecimal(convertCurrency(bigAmount, amountCurrency.code));
        const fixedText = `${symbol}${feeFormated}`;
        return `${percentage}% + ${fixedText}`;
      }
    }
  }

  render() {
    const { profile } = this.props;
    const avatarHash = get(profile, 'avatarHashes.tiny');
    const name = get(profile, 'name');
    const location = get(profile, 'location');
    const description = get(profile, 'moderatorInfo.description');
    return (
      <View style={styles.wrapper}>
        <View style={styles.imgWrapper}>
          <Avatar thumbnail={avatarHash} style={styles.avatarImage} />
        </View>
        <View style={styles.descriptionWrapper}>
          <View style={styles.profileInfo}>
            <Text style={styles.name}>{decode(name)}</Text>
            <View style={styles.verifiedWrapper}>
              <Image style={styles.verifyIcon} source={CheckDecagramIcon} />
              <Text style={styles.verifiedText}>{I18n.t('components.organism.Moderator.verified')} </Text>
            </View>
          </View>
          <Text style={styles.description} numberOfLines={3} ellipsizeMode="tail">
            {eatSpaces(he.decode(striptags.default(description)))}
          </Text>
          <View style={styles.briefContainer}>
            <LocationPin location={location} secondary />
            <FontAwesome style={styles.dollarSign} name="dollar" size={14} />
            <Text style={styles.fee}>{this.getFeeText()}</Text>
          </View>
        </View>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(ModeratorPreview);
