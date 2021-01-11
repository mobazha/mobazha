import React from 'react';
import { connect } from 'react-redux';
import { View, Text, ActivityIndicator, TouchableWithoutFeedback, Image } from 'react-native';
import { withNavigation } from 'react-navigation';
import { get } from 'lodash';
import FastImage from 'react-native-fast-image';
import Feather from 'react-native-vector-icons/Feather';
import decode from 'unescape';

import { fetchProfile } from '../../reducers/profile';
import { getOrderDetails } from '../../api/orders';
import { getOrderText, getDisputeText } from '../../utils/notification';
import { timeSince } from '../../utils/time';

import { convertorsMap } from '../../selectors/currency';
import { getImageSourceWithDefault } from '../../utils/files';
import { getGeneralCoinInfo } from '../../utils/currency';
import { primaryTextColor, brandColor } from '../commonColors';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    borderLeftWidth: 5,
    borderColor: 'transparent',
    minHeight: 72,
    backgroundColor: '#FFFFFF',
    flexDirection: 'row',
    paddingHorizontal: 12,
    paddingVertical: 14,
  },
  loadingWrapper: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  new: {
    backgroundColor: '#F1FBF2',
  },
  iconWrapper: {
    paddingRight: 12,
    paddingTop: 9,
  },
  content: {
    flex: 1,
    flexDirection: 'column',
    justifyContent: 'center',
    paddingRight: 12,
  },
  text: {
    fontSize: 14,
    flex: 1,
    color: primaryTextColor,
  },
  name: {
    fontWeight: '600',
  },
  icon: {
    width: 16,
    height: 16,
  },
  productPreview: {
    flexDirection: 'row',
    marginTop: 4,
  },
  previewImage: {
    width: 35,
    height: 35,
    marginRight: 10,
  },
  productName: {
    fontSize: 14,
    fontWeight: '600',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#777777',
    flex: 1,
  },
  priceWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  coinIcon: {
    width: 16,
    height: 16,
    marginRight: 5,
  },
  price: {
    fontSize: 14,
    fontWeight: '600',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#00BF65',
  },
  timestamp: {
    width: 35,
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'right',
    color: '#8a8a8f',
    alignSelf: 'flex-start',
  },
};

class OrderNotification extends React.PureComponent {
  state = {
    orderDetails: {},
    loading: false,
  };
  componentDidMount() {
    const {
      notification: {
        notification: { orderId },
      },
      username,
      password,
    } = this.props;
    this.setState({
      loading: true,
    });
    getOrderDetails(username, password, orderId).then((response) => {
      const { contract } = response;
      const buyerID = get(contract, 'buyerOrder.buyerID.peerID');
      const vendorID = get(contract, 'vendorListings[0].vendorID.peerID');
      this.props.fetchProfile({ peerID: buyerID, getLoaded: true });
      this.props.fetchProfile({ peerID: vendorID, getLoaded: true });
      if (!get(response, 'success', true) || !contract) {
        this.setState({
          orderDetails: false,
          loading: false,
        });
      } else {
        this.setState({
          orderDetails: response,
          loading: false,
        });
      }
    });
  }

  componentWillUnmount() {
    this.setState({ loading: false });
  }

  getProfile(peerID) {
    const { profiles } = this.props;
    return (profiles && profiles[peerID]) || {};
  }

  getContentText() {
    const {
      notification: {
        type,
        notification: {
          expiresIn, disputeeId, disputerId, otherPartyId,
        },
      },
      profile: { peerID },
      category,
    } = this.props;
    const { orderDetails } = this.state;
    const buyerID = get(orderDetails, 'contract.buyerOrder.buyerID.peerID');
    const buyerDetails = this.getProfile(buyerID);
    const buyerName = get(buyerDetails, 'name', 'Anonymous');
    const vendorID = get(orderDetails, 'contract.vendorListings[0].vendorID.peerID');
    const vendorDetails = this.getProfile(vendorID);
    const vendorName = get(vendorDetails, 'name', 'Anonymous');
    if (category === 'order') {
      return getOrderText(type, buyerName, vendorName, buyerID === peerID);
    } else {
      const disputeeDetail = this.getProfile(disputeeId);
      const disputerDetail = this.getProfile(disputerId);
      const otherPartyDetail = this.getProfile(otherPartyId);
      const moderatorID = get(orderDetails, 'contract.buyerOrder.payment.moderator');
      const moderatorDetails = this.getProfile(moderatorID);
      const moderatorName = get(moderatorDetails, 'name', 'Anonymous');
      const disputeeName = get(disputeeDetail, 'name', 'Anonymous');
      const disputerName = get(disputerDetail, 'name', 'Anonymous');
      const otherPartyName = get(otherPartyDetail, 'name', 'Anonymous');
      const acceptPartyId = get(orderDetails, 'contract.disputeAcceptance.closedBy');
      return getDisputeText(
        type,
        disputeeName,
        disputerName,
        otherPartyName,
        buyerName,
        vendorName,
        moderatorName,
        acceptPartyId === buyerID,
        expiresIn,
      );
    }
  }

  getIcon() {
    const {
      notification: { type },
    } = this.props;
    switch (type) {
      case 'payment':
      case 'orderConfirmation':
      case 'refund':
      case 'fulfillment':
        return (<Feather name="shopping-cart" color={brandColor} size={22} />);
      case 'disputeOpen':
      case 'disputeClose':
      case 'disputeAccepted':
      case 'buyerDisputeExpiry':
      case 'vendorFinalizePayment':
        return (<Feather name="shield" color={brandColor} size={22} />);
      default:
        return (<Feather name="tag" color={brandColor} size={22} />);
    }
  }

  getCoinIcon() {
    const { orderDetails } = this.state;
    const coin = get(orderDetails, 'contract.buyerOrder.payment.amountCurrency.code');
    const coinInfo = getGeneralCoinInfo(coin);
    return get(coinInfo, 'icon', false);
  }

  previewImage() {
    const { orderDetails } = this.state;
    const previewImage = get(orderDetails, 'contract.vendorListings[0].item.images[0].small');
    const imgSource = getImageSourceWithDefault(previewImage);
    return (<FastImage
      style={styles.previewImage}
      source={{
        uri: imgSource.uri,
        priority: FastImage.priority.high,
      }}
      resizeMode={FastImage.resizeMode.contain}
    />);
  }

  gotoDetail = () => {
    const { orderDetails } = this.state;
    const {
      profile: { peerID },
      notification: { notification },
    } = this.props;
    let orderType = 'sales';
    const {
      contract: {
        buyerOrder: {
          buyerID: { peerID: buyerID },
        },
      },
      state: category,
    } = orderDetails;
    if (peerID === buyerID) {
      orderType = 'purchases';
    }
    eventTracker.trackEvent('Notification-TappedNotification', { orderType, category });
    this.props.navigation.navigate({
      routeName: 'OrderDetails',
      params: {
        orderId: notification.orderId,
        orderType,
      },
    });
  }

  render() {
    const {
      notification: { read, timestamp },
      convertCurrency,
      localSymbol,
    } = this.props;
    const { loading } = this.state;
    if (loading) {
      return (
        <View style={[styles.wrapper, styles.loadingWrapper, !read ? styles.new : {}]}>
          <ActivityIndicator size={1} />
        </View>
      );
    }
    const contentText = this.getContentText();
    const { orderDetails } = this.state;
    if (!orderDetails) {
      return false;
    }
    const productTitle = get(orderDetails, 'contract.vendorListings[0].item.title');
    const price = get(orderDetails, 'contract.buyerOrder.payment.bigAmount');
    const pricingCurrency = get(orderDetails, 'contract.buyerOrder.payment.amountCurrency.code');
    const convertedPrice = convertCurrency(price, pricingCurrency).toFixed(2);
    const coinIcon = this.getCoinIcon();
    return (
      <TouchableWithoutFeedback onPress={this.gotoDetail}>
        <View style={[styles.wrapper, !read ? styles.new : {}]}>
          <View style={styles.iconWrapper}>{this.getIcon()}</View>
          <View style={styles.content}>
            <Text style={styles.text}>
              <Text style={styles.name}>{decode(contentText.name)}</Text>
              {contentText.text}
            </Text>
            <View style={styles.productPreview}>
              {this.previewImage()}
              <View style={styles.productBrief}>
                <Text style={styles.productName} numberOfLines={1}>
                  {decode(productTitle)}
                </Text>
                <View style={styles.priceWrapper}>
                  {coinIcon && <Image source={coinIcon} style={styles.coinIcon} />}
                  <Text style={styles.price}>
                    {localSymbol}
                    {convertedPrice}
                  </Text>
                </View>
              </View>
            </View>
          </View>
          <Text style={styles.timestamp}>{timeSince(new Date(timestamp).getTime())}</Text>
        </View>
      </TouchableWithoutFeedback>
    );
  }
}

const mapStateToProps = state => ({
  profiles: state.profiles,
  profile: state.profile.data,
  username: state.appstate.username,
  password: state.appstate.password,
  ...convertorsMap(state),
});
const mapDispatchToProps = {
  fetchProfile,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
)(OrderNotification));
