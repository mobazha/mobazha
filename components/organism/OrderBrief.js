import React from 'react';
import { connect } from 'react-redux';
import { View, Text, TouchableWithoutFeedback, Image } from 'react-native';
import { get } from 'lodash';
import { withNavigation } from 'react-navigation';
import FastImage from 'react-native-fast-image';
import decode from 'unescape';

import { getImageSourceWithDefault } from '../../utils/files';
import { getStatusInfo } from '../../utils/order';

import { primaryTextColor, secondaryTextColor, borderColor, formLabelColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';
import { COINS } from '../../utils/coins';
import { timeSince } from '../../utils/time';
import { fetchProfile } from '../../reducers/profile';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    borderTopWidth: 1,
    borderBottomWidth: 1,
    borderColor,
    backgroundColor: 'white',
  },
  unread: {
    backgroundColor: 'rgba(0, 191, 16, 0.05)',
  },
  innerWrapper: {
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  image: {
    width: 115,
    height: 115,
  },
  mainContent: {
    alignSelf: 'stretch',
    flex: 1,
    flexDirection: 'row',
    paddingVertical: 6,
    paddingLeft: 11,
  },
  orderInfo: {
    flexDirection: 'column',
    flex: 1,
    justifyContent: 'space-between',
  },
  timeInfo: {
    marginHorizontal: 11,
    fontSize: 13,
    fontStyle: 'normal',
    letterSpacing: 0,
    color: formLabelColor,
  },
  itemTitle: {
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  secondPartyInfo: {
    fontSize: 13,
    marginBottom: 7,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: secondaryTextColor,
  },
  learnMore: {
    paddingVertical: 5,
    fontSize: 10,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: secondaryTextColor,
  },
  priceWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 8,
  },
  icon: {
    width: 14,
    height: 14,
  },
  priceText: {
    marginLeft: 4,
    fontSize: 14,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#009402',
  },
  button: {
    paddingVertical: 3,
    paddingHorizontal: 6,
    borderRadius: 2,
    borderWidth: 1,
    borderColor: formLabelColor,
    alignSelf: 'flex-start',
    backgroundColor: 'white',
  },
  buttonText: {
    fontSize: 11,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  underline: {
    textDecorationLine: 'underline',
  },
  contentContainer: {
    flex: 1,
  },
  orderStatusWrapper: {
    height: 30,
    paddingHorizontal: 11,
    paddingBottom: 6,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  orderNumber: {
    fontSize: 12,
    color: formLabelColor,
    textAlign: 'right',
  },
};

class OrderBrief extends React.Component {
  static getDerivedStateFromProps(props, state) {
    const { profiles, order } = props;
    const peerID = get(order, 'vendorId') || get(order, 'buyerId');
    return { profile: profiles && profiles[peerID] };
  }

  state = {}

  componentDidMount() {
    const { fetchProfile, order, shouldFetchProfile } = this.props;
    const peerID = get(order, 'vendorId') || get(order, 'buyerId');
    if (shouldFetchProfile) {
      fetchProfile({ peerID, getLoaded: true });
    }
  }

  shouldComponentUpdate(nextProps, nextState) {
    const { profile } = this.state;
    if ((!profile && nextState.profile)
      || (this.props.order !== nextProps.order)
      || (this.props.orderType !== nextProps.orderType)
    ) {
      return true;
    } else {
      return false;
    }
  }

  getSecondPartyLabel = () => {
    const { orderType } = this.props;
    const { profile = {} } = this.state;
    return `${orderType === 'purchases' ? I18n.t('components.organism.OrderBrief.from') : I18n.t('components.organism.OrderBrief.to')} ${decode(profile.name) || 'Unknown'}`;
  }

  renderStatus = () => {
    const { order = {}, orderType, showOrderId } = this.props;
    const { state } = order;
    if (!order.orderId) {
      return null;
    }
    const info = getStatusInfo(state, orderType);
    const label = info ? info.label : state;
    return (
      <View style={styles.orderStatusWrapper}>
        <View style={styles.button}>
          <Text style={styles.buttonText}>{label.toUpperCase()}</Text>
        </View>
        {showOrderId && <Text style={styles.orderNumber} numberOfLines={1}>#{order.orderId.substring(0, 5)}...</Text>}
      </View>
    );
  }

  render() {
    const {
      localLabelFromBCH,
      order,
      onPress,
      noBorder,
      topBorder,
      highlightUnread,
      hideTimestamp,
      showLearnMore,
      onLearnMore,
    } = this.props;
    const {
      thumbnail, timestamp, title, total, orderId, read, paymentCoin,
    } = order;
    return (
      <View
        style={[
          styles.wrapper,
          !topBorder && { borderTopWidth: 0 },
          noBorder && { borderBottomWidth: 0 },
          highlightUnread && !read && styles.unread,
        ]}
      >
        <TouchableWithoutFeedback
          onPress={() => {
            if (onPress) { onPress(orderId); }
          }}
        >
          <View style={styles.innerWrapper}>
            <FastImage
              source={getImageSourceWithDefault(thumbnail)}
              style={styles.image}
            />
            <View style={styles.contentContainer}>
              <View style={styles.mainContent}>
                <View style={styles.orderInfo}>
                  <Text style={styles.itemTitle} numberOfLines={1}>{title}</Text>
                  <Text style={styles.secondPartyInfo} numberOfLines={1}>
                    {this.getSecondPartyLabel()}
                  </Text>
                  <View style={styles.priceWrapper}>
                    <Image style={styles.icon} source={COINS[paymentCoin].icon} resizeMode="cover" />
                    <Text style={styles.priceText}>{`${localLabelFromBCH(total.amount, paymentCoin)}`}</Text>
                  </View>
                  {showLearnMore && (
                    <TouchableWithoutFeedback onPress={onLearnMore}>
                      <View>
                        <Text style={styles.learnMore}>
                          {I18n.t('components.organism.OrderBrief.tap_to')}<Text style={styles.underline}>{I18n.t('components.organism.OrderBrief.learn_more')}</Text>.
                        </Text>
                      </View>
                    </TouchableWithoutFeedback>
                  )}
                </View>
                {!hideTimestamp && (
                  <Text style={styles.timeInfo}>{timeSince(new Date(timestamp).getTime())}</Text>
                )}
              </View>
              {this.renderStatus()}
            </View>
          </View>
        </TouchableWithoutFeedback>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  fetchProfile,
};

export default withNavigation(connect(mapStateToProps, mapDispatchToProps)(OrderBrief));
