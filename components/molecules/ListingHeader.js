import React, { PureComponent } from 'react';
import { Platform, View, TouchableWithoutFeedback } from 'react-native';
import Ionicons from 'react-native-vector-icons/Ionicons';
import Feather from 'react-native-vector-icons/Feather';
import MaterialIcons from 'react-native-vector-icons/MaterialIcons';

import { backgroundColor } from '../commonColors';
import NavBackButton from '../atoms/NavBackButton';
import StatusBarWrapper from '../../status-bar';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    zIndex: 9999,
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
  },
  contentWrapper: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    width: '100%',
    paddingTop: 5,
    paddingLeft: 6,
    paddingRight: 6,
  },
  btnWrapper: {
    flexDirection: 'row',
    padding: 4,
  },
  btn: {
    backgroundColor: 'rgba(108, 108, 108, 0.5)',
    width: 40,
    height: 40,
    borderRadius: 20,
    justifyContent: 'center',
    alignItems: 'center',
  },
  listingBtnOptions: {
    paddingTop: Platform.OS === 'ios' ? 4 : 2,
    paddingLeft: Platform.OS === 'ios' ? 1 : 0,
  },
  extraLeftSpace: {
    marginRight: 5,
  },
  addPhoto: {
    paddingBottom: 4,
  },
  androidShareIcon: {
    paddingRight: 3,
  },
};

export default class ListingHeader extends PureComponent {
  handleToggleWishlist = () => {
    const { wishlist, action } = this.props;
    action(!wishlist);
  }

  handleShare = () => {
    this.props.onShare();
  }

  renderStarIcon = () => {
    const { editing, wishlist } = this.props;
    if (editing) {
      return <MaterialIcons style={styles.addPhoto} name="add-a-photo" color={backgroundColor} size={24} />;
    } else {
      return (
        <Ionicons
          name={wishlist ? 'md-heart' : 'md-heart-empty'}
          size={25}
          color={wishlist ? 'rgb(255, 59, 48)' : backgroundColor}
        />
      );
    }
  }

  renderShareIcon = () => (
    <Feather
      style={Platform.OS === 'ios' ? {} : styles.androidShareIcon}
      name={Platform.OS === 'ios' ? 'share' : 'share-2'}
      size={25}
      color={backgroundColor}
    />
  )

  render() {
    const { showActions, onBack, onMore, onShare } = this.props;
    return (
      <View style={styles.wrapper}>
        <StatusBarWrapper backgroundColor="white" barStyle="dark-content" />
        <View style={styles.contentWrapper}>
          <View style={styles.btnWrapper}>
            <TouchableWithoutFeedback onPress={onBack}>
              <View style={styles.btn}>
                <NavBackButton white />
              </View>
            </TouchableWithoutFeedback>
          </View>
          {showActions && (
            <View style={styles.btnWrapper}>
              <TouchableWithoutFeedback onPress={this.handleToggleWishlist}>
                <View style={[styles.btn, styles.listingBtnOptions, styles.extraLeftSpace]}>
                  {this.renderStarIcon()}
                </View>
              </TouchableWithoutFeedback>
              {onShare && (
                <TouchableWithoutFeedback onPress={this.handleShare}>
                  <View style={[styles.btn, styles.extraLeftSpace]}>
                    {this.renderShareIcon()}
                  </View>
                </TouchableWithoutFeedback>
              )}
              <TouchableWithoutFeedback onPress={onMore}>
                <View style={[styles.btn, styles.listingBtnOptions]}>
                  <Ionicons name="md-more" size={30} color={backgroundColor} />
                </View>
              </TouchableWithoutFeedback>
            </View>
          )}
        </View>
      </View>
    );
  }
}
