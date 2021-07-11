import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Clipboard, Text, View, Alert, StyleSheet, ScrollView, RefreshControl } from 'react-native';
import { BlurView } from '@react-native-community/blur';
import { NavigationActions, NavigationEvents, withNavigation } from 'react-navigation';
import Dialog from 'react-native-dialog';
import { get, hasIn, isEmpty } from 'lodash';
import Foundation from 'react-native-vector-icons/Foundation';
import deepEqual from 'deep-equal';

import NavCloseButton from '../components/atoms/NavCloseButton';
import Header from '../components/molecules/Header';
import RatingModal from '../components/templates/RatingModal';
import OrderSummary from '../components/templates/OrderSummary';
import { OBLightModal } from '../components/templates/OBModal';
import { screenWrapper } from '../utils/styles';
import Tabs from '../components/organism/Tabs';
import StoreMoreMenu from '../components/molecules/StoreMoreMenu';

import { fetchProfile } from '../reducers/profile';
import * as orderActions from '../reducers/order';
import { getUserPeerID } from '../selectors/profile';
import { getOrderTitleFromOrderId, getOrderBriefFromDetails } from '../utils/order';
import { refundOrder as doRefund } from '../api/orders';
import NavBackButton from '../components/atoms/NavBackButton';
import OrderBrief from '../components/organism/OrderBrief';
import { formLabelColor, foregroundColor, borderColor } from '../components/commonColors';
import ChatDetailTemplate from '../components/templates/ChatDetail';
import OrderFooter from '../components/organism/OrderFooter';
import { ordersMap } from '../selectors/order';
import FulfillModal from '../components/templates/FulfillModal';
import DisputeModal from '../components/templates/DisputeModal';
import OBActionSheet from '../components/organism/ActionSheet';
import { eventTracker } from '../utils/EventTracker';
import ContractModal from '../components/templates/ContractModal';

import {I18n} from '../langs/I18n';

const styles = {
  placeholderWrapper: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  placeholderText: {
    width: 298.5,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: '#8a8a8f',
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 48,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 240,
    top: 200,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
  brief: {
    paddingHorizontal: 17,
    paddingTop: 17,
  },
  contentContainer: {
    flex: 1,
    backgroundColor: foregroundColor,
  },
  learnMoreText: {
    fontSize: 14,
    padding: 16,
  },
};

class OrderDetails extends PureComponent {
  state = {
    copied: false,
    tab: 'details',
    orderId: '',
    showFundDlg: false,
    showDisputeDlg: false,
    showFulfillDlg: false,
    showRatingModal: false,
    showLearnMoreModal: false,
    contractType: 'SERVICE',
    paymentAddress: '',
    requestedAmount: 0,
    memo: '',
    disputeReason: '',
    orderDetails: {},
    updating: false,
    showContractModal: false,
  };

  componentDidMount() {
    this.updateInfo();
    this.markOrderAsRead();
  }

  componentDidUpdate(prevProps) {
    const {
      orderDetails, navigation, fetchProfile, fetching,
    } = this.props;
    const orderId = this.getOrderId();
    const orderType = navigation.getParam('orderType');

    if (!isEmpty(orderDetails) && !deepEqual(orderDetails, prevProps.orderDetails)) {
      this.setState({
        order: getOrderBriefFromDetails(orderId, orderDetails, orderType),
        orderDetails,
      });
      const moderator = get(orderDetails, 'contract.buyerOrder.payment.moderator');
      fetchProfile({ peerID: moderator, getLoaded: true });
    }

    if (this.isNavigationUpdated(prevProps.navigation.state)) {
      this.updateInfo();
    }

    if (fetching && !prevProps.fetching) {
      this.setState({ updating: false });
    }
  }

  onRefresh = () => {
    this.updateInfo();
    this.setState({ updating: true });
  }

  onOrderAction = (action, total) => {
    const { order = {} } = this.state;
    const orderId = this.getOrderId();
    const vendorID = this.getVendorId();
    const {
      confirmOrder, refundOrder,
    } = this.props;
    switch (action) {
      case 'Pay':
        this.fundOrder(total);
        break;
      case 'Dispute':
        this.disputeOrder();
        break;
      case 'Claim':
        this.claimOrder();
        break;
      case 'Complete':
        this.showRatingModal();
        break;
      case 'Write a review':
        this.showRatingModal();
        break;
      case 'Fulfill': {
        const { orderDetails } = this.state;
        const vendorListings = get(orderDetails, 'contract.vendorListings', []);
        const contractType = get(vendorListings[0], 'metadata.contractType');
        this.setState({ contractType, showFulfillDlg: true });
        break;
      }
      case 'Accept':
        confirmOrder({ orderId, reject: false, peerID: order.buyerId });
        break;
      case 'Decline':
        Alert.alert(
          I18n.t('screens.orderDetails.decline_order'),
          I18n.t('screens.orderDetails.decline_hin'),
          [
            { text:I18n.t('screens.orderDetails.nevermindr')},
            {
              text: 'Ok',
              onPress: () => {
                confirmOrder({ orderId, reject: true, peerID: vendorID });
              },
            },
          ],
        );
        break;
      case 'Cancel':
        this.cancelOrder();
        break;
      case 'Refund':
        Alert.alert(
          I18n.t('screens.orderDetails.refund_order'),
          I18n.t('screens.orderDetails.refund_hint'),
          [
            { text: I18n.t('screens.orderDetails.nevermindr') },
            {
              text: 'Ok',
              onPress: () => {
                refundOrder({ orderId, vendorId: order.buyerId });
              },
            },
          ],
        );
        break;
      default:
        break;
    }
  }

  getOrderId = () => {
    const { navigation } = this.props;
    const orderFromParam = navigation.getParam('order');
    const orderIdFromParam = navigation.getParam('orderId');
    let orderId;
    if (orderFromParam) {
      orderId = orderFromParam.orderId;
    } else if (orderIdFromParam) {
      orderId = orderIdFromParam;
    }
    return orderId;
  }

  getVendorId = () => {
    const { orderDetails } = this.props;
    return get(orderDetails, 'contract.vendorListings[0].vendorID.peerID');
  }

  getOrderTabs = () => [
    { value: 'details', label: I18n.t('screens.orderDetails.details') },
    {
      value: 'discussion',
      label: I18n.t('screens.orderDetails.discussion'),
    },
  ]

  setActionSheet = (component) => {
    this.actionSheet = component;
  };

  markOrderAsRead = () => {
    const { navigation, updatePurchase, updateSale } = this.props;

    const orderType = navigation.getParam('orderType');
    const orderId = this.getOrderId();

    const payload = { orderId, body: { read: true } };
    if (orderType === 'purchases') {
      updatePurchase(payload);
    } else {
      updateSale(payload);
    }
  }

  isNavigationUpdated = (navState) => {
    const { navigation: { state } } = this.props;
    return !deepEqual(state.params, navState.params);
  }

  updateInfo = () => {
    const {
      getOrderBrief, navigation, fetchOrder, clearOrder,
    } = this.props;

    const orderType = navigation.getParam('orderType');
    const orderFromParam = navigation.getParam('order');
    const orderDetailsFromParam = navigation.getParam('orderDetails', false);
    const orderIdFromParam = navigation.getParam('orderId');
    const orderId = this.getOrderId();

    let order;
    if (orderFromParam) {
      order = orderFromParam;
    } else if (orderIdFromParam) {
      order = getOrderBrief(orderIdFromParam);
    }

    clearOrder();
    if (!orderDetailsFromParam) {
      this.setState({ order });

      fetchOrder(orderId);
    } else {
      this.setState({
        order: getOrderBriefFromDetails(orderId, orderDetailsFromParam, orderType),
        orderDetails: orderDetailsFromParam,
      });
    }
  }

  showRatingModal = () => this.setState({ showRatingModal: true })
  hideRatingModal = () => this.setState({ showRatingModal: false })

  doComplete = (orderId, ratings) => {
    const vendorId = this.getVendorId();
    this.hideRatingModal();
    setTimeout(() => {
      this.props.completeOrder({ orderId, ratings, vendorId });
    }, 500);
  }

  fundOrder = (total) => {
    const { balance } = this.props;
    const contract = get(this.state, 'orderDetails.contract');
    if (!contract) {
      return;
    }

    const { vendorOrderConfirmation, buyerOrder } = contract;
    const { paymentAddress } = vendorOrderConfirmation || {};
    const {
      address: paymentAddress1, bigAmount: requestedAmount, amountCurrency,
    } = buyerOrder.payment;
    const { code: coin } = amountCurrency;

    if (total > balance[coin].confirmed) {
      Alert.alert('Insufficient funds');
    } else {
      this.setState({
        coin,
        paymentAddress: paymentAddress || paymentAddress1,
        requestedAmount,
        showFundDlg: true,
      });
    }
  }

  doFund = () => {
    const {
      order, paymentAddress, requestedAmount, memo, coin,
    } = this.state;
    const { orderId } = order;
    const peerID = this.getVendorId();
    this.setState({ showFundDlg: false });

    setTimeout(() => {
      const { fundOrder } = this.props;
      fundOrder({
        coin,
        address: paymentAddress,
        amount: requestedAmount,
        memo,
        orderId,
        peerID,
      });
    }, 500);
  }

  disputeOrder = () => {
    this.setState({ showDisputeDlg: true });
  }

  claimOrder = () => {
    const orderId = this.getOrderId();
    const vendorId = this.getVendorId();
    const { claimPayment } = this.props;
    claimPayment({ orderId, vendorId });
  }

  cancelOrder = () => {
    const orderId = this.getOrderId();
    const vendorID = this.getVendorId();
    const { cancelOrder } = this.props;
    Alert.alert(
      I18n.t('screens.orderDetails.cancel_order'),
      I18n.t('screens.orderDetails.cancel_hint'),
      [
        { text:  I18n.t('screens.orderDetails.nevermindr') },
        {
          text: 'Ok',
          onPress: () => cancelOrder({ orderId, vendorID }),
        },
      ],
    );
  }

  refundOrder = (orderId) => {
    const { username, password } = this.props;
    doRefund(username, password, orderId).then((response) => {
      if (hasIn(response, 'success') && response.success === false) {
        Alert.alert(response.reason);
      } else {
        Alert.alert(I18n.t('screens.orderDetails.have_refunded'));
        this.updateOrderList();
      }
    }).catch((err) => {
      Alert.alert(I18n.t('screens.orderDetails.error_happened'));
    });
  }

  updateOrderList = () => {
    const orderType = this.props.navigation.getParam('orderType');
    switch (orderType) {
      case 'sales':
        this.props.fetchSales();
        break;
      case 'purchases':
        this.props.fetchPurchases();
        break;
      default:
        break;
    }
  }

  updateState = field => (value) => {
    const dict = {};
    dict[field] = value;
    this.setState(dict);
  }

  handleGoToChat = () => {
    eventTracker.trackEvent('Order-ViewDetailsDiscussion');
    this.updateState('tab')('discussion');
  }

  handleGoToListing = () => {
    const { navigation, profile } = this.props;
    const orderType = navigation.getParam('orderType');
    const { order } = this.state;

    navigation.navigate({
      routeName: 'Listing',
      params: {
        slug: get(order, 'slug'),
        peerID: orderType === 'purchases' ? get(order, 'vendorId') : profile.peerID,
      },
    });
  }

  handleChange = (index) => {
    const { getPeerIDFromOrder, navigation } = this.props;
    const orderType = navigation.getParam('orderType');
    const peerID = getPeerIDFromOrder(this.getOrderId(), orderType);
    switch (index) {
      case 0: {
        navigation.navigate('ExternalStore', { peerID });
        break;
      }
      case 1:
        this.handleGoToListing();
        break;
      case 2: {
        Clipboard.setString(this.getOrderId());

        this.setState({ copied: true });
        setTimeout(() => this.setState({ copied: false }), 2000);
        break;
      }
      case 3: {
        this.setState({ showContractModal: true });
        break;
      }
      default: {
        break;
      }
    }
  };

  handlePressMore = () => {
    this.actionSheet.show();
  }

  handleHideFulfill = () => {
    this.setState({ showFulfillDlg: false });
  }

  handleHideDispute = () => {
    this.setState({ showDisputeDlg: false });
  }

  showLearnMoreDlg = () => {
    this.setState({ showLearnMoreModal: true });
  }

  hideLearnMoreDlg = () => {
    this.setState({ showLearnMoreModal: false });
  }

  handleNavigationFocus = () => {
    const { navigation } = this.props;
    const { tab } = this.state;
    this.setState({ tab: navigation.getParam('tab') || tab });
  }

  handleCloseContractModal = () => {
    this.setState({ showContractModal: false });
  }

  renderEmptyState = () => (
    <View style={styles.placeholderWrapper}>
      <Foundation name="price-tag" size={50} color={formLabelColor} />
      <Text style={styles.placeholderText}>({I18n.t('screens.orderDetails.order_discussions')})</Text>
    </View>
  );

  render() {
    const {
      getPeerIDFromOrder, navigation, peerID, fetching,
    } = this.props;
    const orderType = navigation.getParam('orderType');
    const orderId = this.getOrderId();

    const header = (
      <Header
        left={<NavBackButton />}
        onLeft={() => {
          navigation.dispatch(NavigationActions.back());
        }}
        title={getOrderTitleFromOrderId(orderId)}
        right={<StoreMoreMenu onMore={this.handlePressMore} black size={28} />}
      />
    );

    const {
      showFundDlg,
      showDisputeDlg,
      showFulfillDlg,
      showRatingModal,
      showLearnMoreModal,
      contractType,
      memo,
      orderDetails = {},
      tab,
      copied,
      order = {},
      updating,
      showContractModal,
    } = this.state;
    if (!order) {
      return (
        <View style={screenWrapper.wrapper}>
          {header}
        </View>
      );
    }

    const moderatorId = get(orderDetails, 'contract.buyerOrder.payment.moderator');
    const isModerated = get(orderDetails, 'contract.buyerOrder.payment.method') === 'MODERATED';
    const disputeTimestamp = get(orderDetails, 'contract.dispute.timestamp');
    const vendorId = order.vendorId || peerID;
    const buyerId = order.buyerId || peerID;
    const blurComponentIOS = (
      <BlurView style={StyleSheet.absoluteFill} blurType="xlight" blurAmount={50} />
    );
    return (
      <View style={screenWrapper.wrapper}>
        <NavigationEvents onDidFocus={this.handleNavigationFocus} />
        {header}
        <Tabs
          currentTab={tab}
          tabs={this.getOrderTabs()}
          onChange={this.updateState('tab')}
          withBorder
        />
        {tab === 'details' ? (
          <ScrollView
            refreshControl={
              <RefreshControl
                refreshing={updating && fetching}
                onRefresh={this.onRefresh}
              />
            }
          >
            {!isEmpty(order) && (
              <View style={styles.brief}>
                <OrderBrief
                  orderType={orderType}
                  order={order}
                  onPress={this.handleGoToListing}
                  noBorder
                  hideTimestamp
                  shouldFetchProfile
                  showLearnMore
                  onLearnMore={this.showLearnMoreDlg}
                />
              </View>
            )}
            {!isEmpty(orderDetails) && (
              <OrderSummary
                orderId={this.getOrderId()}
                orderDetails={orderDetails}
                orderType={orderType}
                onDispute={this.disputeOrder}
                onClaim={this.claimOrder}
                onCancel={this.cancelOrder}
                onMessage={this.handleGoToChat}
                navigation={navigation}
              />
            )}
          </ScrollView>
        ) : (
          <ChatDetailTemplate
            isFromOrder
            isModerated={isModerated}
            peerID={getPeerIDFromOrder(this.getOrderId(), orderType)}
            subject={this.getOrderId()}
            orderType={orderType}
            moderatorId={moderatorId}
            vendorId={vendorId}
            buyerId={buyerId}
            inDispute={orderDetails.state === 'DISPUTED'}
            disputeTimestamp={disputeTimestamp}
          />
          )}
        {tab === 'details' && (
          <OrderFooter
            orderType={orderType}
            order={order}
            orderDetails={orderDetails}
            onOrderAction={this.onOrderAction}
          />
        )}
        <Dialog.Container visible={showFundDlg} blurComponentIOS={blurComponentIOS}>
          <Dialog.Title>{I18n.t('screens.orderDetails.fund_order')}</Dialog.Title>
          <Dialog.Description>{I18n.t('screens.orderDetails.leave_notes')}</Dialog.Description>
          <Dialog.Input value={memo} onChangeText={this.updateState('memo')} />
          <Dialog.Button
            label="Cancel"
            onPress={() => {
              this.setState({ showFundDlg: false, memo: '' });
            }}
          />
          <Dialog.Button label="Confirm" onPress={this.doFund} />
        </Dialog.Container>
        <FulfillModal
          isOpen={showFulfillDlg}
          orderId={this.getOrderId()}
          buyerId={buyerId}
          vendorId={this.getVendorId()}
          contractType={contractType}
          onHideFulfill={this.handleHideFulfill}
        />
        <DisputeModal
          orderType={orderType}
          isOpen={showDisputeDlg}
          orderId={this.getOrderId()}
          vendorId={this.getVendorId()}
          onHideDispute={this.handleHideDispute}
        />
        <RatingModal
          show={showRatingModal}
          orderType={orderType}
          order={order}
          hideModal={this.hideRatingModal}
          finishReview={this.doComplete}
        />
        <OBActionSheet
          ref={this.setActionSheet}
          onPress={this.handleChange}
          options={[
            orderType === 'purchases' ? I18n.t('screens.orderDetails.go_to_seller_profile') : I18n.t('screens.orderDetails.go_to_buyer_profile'),
            I18n.t('screens.orderDetails.view_listing'),
            I18n.t('screens.orderDetails.copy_order_number'),
            I18n.t('screens.orderDetails.view_contract'),
            I18n.t('screens.orderDetails.cancel'),
          ]}
          cancelButtonIndex={4}
        />
        {copied && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{I18n.t('screens.orderDetails.number_copied')}</Text>
          </View>
        )}
        <OBLightModal
          animationType="slide"
          transparent
          visible={showLearnMoreModal}
          onRequestClose={this.hideLearnMoreDlg}
        >
          <Header
            modal
            left={<NavCloseButton />}
            onLeft={this.hideLearnMoreDlg}
          />
          <View style={styles.contentContainer}>
            <Text style={styles.learnMoreText}>
            {I18n.t('screens.orderDetails.learn_more')}.
            </Text>
          </View>
        </OBLightModal>
        <ContractModal
          show={showContractModal}
          onClose={this.handleCloseContractModal}
          orderDetails={orderDetails}
        />
      </View>
    );
  }
}

const mapStateToProps = state => ({
  profile: state.profile.data,
  peerID: getUserPeerID(state),
  sales: state.orders.sales,
  fetching: state.orders.fetching,
  purchases: state.orders.purchases,
  orderDetails: state.orders.orderDetails,
  balance: state.wallet.balance,
  ...ordersMap(state),
});

const mapDispatchToProps = {
  ...orderActions,
  fetchProfile,
};

export default withNavigation(connect(
  mapStateToProps,
  mapDispatchToProps,
)(OrderDetails));
