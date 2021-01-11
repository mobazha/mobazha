import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Image, Dimensions, View, Text, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';

import {
  foregroundColor,
  borderColor,
  primaryTextColor,
  formLabelColor,
} from '../commonColors';
import { priceStyle } from '../commonStyles';
import { getImageSourceWithDefault } from '../../utils/files';
import { convertorsMap } from '../../selectors/currency';

import {I18n} from '../../langs/I18n';

const { width: SCREEN_WIDTH } = Dimensions.get('window');

const styles = {
  wrapper: {
    marginTop: 12,
    marginLeft: 12,
    width: SCREEN_WIDTH - 88,
    height: SCREEN_WIDTH - 40,
    borderRadius: 4,
    backgroundColor: foregroundColor,
    borderWidth: 1,
    borderColor,
    overflow: 'hidden',
  },
  marginedWrapper: {
    height: SCREEN_WIDTH - 48,
    borderRadius: 4,
    backgroundColor: foregroundColor,
    borderWidth: 1,
    borderColor,
    marginRight: 11,
    marginTop: 11,
    overflow: 'hidden',
  },
  image: {
    width: '100%',
    flex: 1,
  },
  title: {
    fontSize: 16,
    color: primaryTextColor,
    fontWeight: 'bold',
    paddingTop: 8,
    paddingLeft: 8,
  },
  price: {
    fontSize: 16,
    fontWeight: 'bold',
    paddingTop: 2,
    paddingLeft: 8,
    paddingBottom: 6,
  },
  btn: {
    position: 'absolute',
    width: 28,
    height: 28,
    borderRadius: 14,
    justifyContent: 'center',
    alignItems: 'center',
    elevation: 3,
    right: 0,
    top: 0,
    backgroundColor: formLabelColor,
  },
  icon: {
    width: 22,
    height: 22,
    textAlign: 'center',
  },
};

class ListingPreview extends PureComponent {
  handleRemove = () => {
    const { onRemove } = this.props;
    onRemove();
  }

  renderContent = (addMargin = false) => {
    const { thumbnail, style, title, currencyCode, amount, localLabelFromBCH } = this.props;
    return (
      <View style={[addMargin ? styles.marginedWrapper : styles.wrapper, style]}>
        <Image
          style={styles.image}
          source={getImageSourceWithDefault(thumbnail)}
          resizeMode="cover"
        />
        <Text style={styles.title} numberOfLines={1}>{title}</Text>
        <Text style={[priceStyle, styles.price]}>
          {amount ? localLabelFromBCH(amount, currencyCode) : ''}
        </Text>
      </View>
    );
  }

  render() {
    const { onRemove, onPress, isInFeedPreview } = this.props;
    if (onRemove) {
      return (
        <View style={styles.container}>
          {this.renderContent(true)}
          <TouchableWithoutFeedback onPress={onRemove}>
            <View style={styles.btn}>
              <Ionicons style={styles.icon} name="md-close" size={22} color={foregroundColor} />
            </View>
          </TouchableWithoutFeedback>
        </View>
      );
    } else {
      return (
        <TouchableWithoutFeedback activeOpacity={1} onPress={onPress}>
          {this.renderContent(isInFeedPreview)}
        </TouchableWithoutFeedback>
      );
    }

  }
}

const mapStateToProps = state => ({
  listings: state.listings.data,
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(ListingPreview);
