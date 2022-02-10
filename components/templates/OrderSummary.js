import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Alert, TouchableWithoutFeedback, Text, View, Image, Linking, Clipboard } from 'react-native';
import { isEmpty, get } from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import Dash from 'react-native-dash';
import decode from 'unescape';
import BigNumber from 'bignumber.js';

import InputGroup from '../atoms/InputGroup';
import ListingTitlePrice from '../atoms/ListingTitlePrice';
import OrderRating from '../organism/OrderRating';
import OrderDispute from '../organism/OrderDispute';
import OrderFulfillment from '../organism/OrderFulfillment';
import Moderator from '../organism/Moderator';
import { convertorsMap } from '../../selectors/currency';

import {
  primaryTextColor,
  secondaryTextColor,
  linkTextColor,
  formLabelColor,
  brandColor,
  foregroundColor,
  borderColor,
  warningColor,
} from '../commonColors';

import { COINS, transactionLinkDict } from '../../utils/coins';
import { timeSince, timestampComparator, timeUntil } from '../../utils/time';
import { parseCountryName } from '../../utils/string';
import { timeSinceOrderTimeout, getOrderBriefFromDetails, isFulfilled } from '../../utils/order';
import { minUnitAmountToBCH, getFixedCurrency, getDecimalPoints } from '../../utils/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  optionWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    flex: 1,
    paddingVertical: 12,
  },
  optionLabel: {
    flex: 1,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  estDelivery: {
    fontSize: 13,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: secondaryTextColor,
  },
  price: {
    fontSize: 14,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: '#00BF65',
  },
  optionPlaceholder: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'italic',
    lineHeight: 19,
    letterSpacing: 0,
    textAlign: 'left',
    color: '#525252',
  },
  paymentMethod: {
    marginTop: 12,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-start',
  },
  paymentLabel: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  coinImage: {
    width: 20,
    height: 20,
  },
  coinName: {
    marginLeft: 5,
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  totalWrapper: {
    paddingVertical: 12,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  totalLabel: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  totalValue: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: brandColor,
  },
  addrWrapper: {
    paddingVertical: 12,
  },
  addrName: {
    fontSize: 15,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  addrLine: {
    fontSize: 15,
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
  },
  addrNote: {
    marginTop: 6,
    fontSize: 15,
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: secondaryTextColor,
  },
  noMemoStyle: {
    fontSize: 15,
    fontWeight: 'normal',
    fontStyle: 'italic',
    letterSpacing: 0,
    textAlign: 'left',
    color: '#8a8a8f',
    paddingVertical: 12,
  },
  memoText: {
    fontSize: 15,
    fontWeight: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    color: primaryTextColor,
    paddingVertical: 12,
  },
  viewTransaction: {
    marginLeft: 8,
    fontSize: 15,
    fontWeight: 'normal',
    color: linkTextColor,
  },
  unpaidNotice: {
    marginVertical: 20,
    fontSize: 14,
    textAlign: 'center',
    color: '#ff001f',
  },
  description: {
    fontSize: 15,
    color: primaryTextColor,
    lineHeight: 18,
    marginVertical: 12,
  },
  escrowDays: {
    fontWeight: 'bold',
  },
  fullButton: {
    width: '100%',
    paddingHorizontal: 17,
    paddingVertical: 12,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
    marginBottom: 20,
  },
  disabledDisputeButton: {
    borderColor,
  },
  fullText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  disabledDisputeText: {
    opacity: 0.4,
  },
  claimButton: {
    width: '100%',
    paddingHorizontal: 17,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: brandColor,
    justifyContent: 'center',
    marginTop: 14,
    marginBottom: 18,
  },
  claimText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: 'white',
  },
  overlay: {
    alignSelf: 'center',
    backgroundColor: 'rgba(34, 34, 34, 0.9)',
    borderRadius: 30,
    height: 40,
    justifyContent: 'center',
    paddingHorizontal: 25,
    position: 'absolute',
    width: 200,
    bottom: 150,
  },
  overlayText: {
    color: '#fff',
    fontSize: 15,
    lineHeight: 18,
    fontWeight: 'normal',
    textAlign: 'center',
  },
  timestamp: {
    color: '#9b9b9b',
  },
  moderator: {
    paddingHorizontal: 17,
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderColor,
  },
  option: {
    fontSize: 13,
    letterSpacing: 0,
    textAlign: 'left',
    color: formLabelColor,
    marginBottom: 8,
  },
  paddedBottom: {
    marginHorizontal: 17,
  },
};

class OrderSummary extends PureComponent {
  static getDerivedStateFromProps(props, state) {
    const paymentAddressTransactions = get(props, 'orderDetails.paymentAddressTransactions');
    if (!paymentAddressTransactions) {
      return { paymentAddressTransactions: [] };
    }
    const transactions = paymentAddressTransactions.slice();
    transactions.sort(timestampComparator);
    return { paymentAddressTransactions: transactions.reverse() };
  }

  state = { copied: false };

  getShippingDetails() {
    const {
      buyerOrder: { items },
      vendorListings,
    } = this.props.orderDetails.contract;

    const { shippingOption } = items[0];
    const { shippingOptions } = vendorListings[0];
    const pickedOption = shippingOptions.find(op => op.name === shippingOption.name);
    if (!pickedOption) {
      return null;
    }
    const { services } = pickedOption;
    const service = services.find(s => s.name === shippingOption.service);
    if (!service) {
      return null;
    }
    return {
      ...service,
      optionName: shippingOption.name,
    };
  }

  getTimeSinceTimeout = () => {
    const { orderDetails, orderId, orderType } = this.props;
    if (!orderDetails) {
      return null;
    }

    const order = getOrderBriefFromDetails(orderId, orderDetails, orderType);
    if (!order) {
      return null;
    }

    return timeSinceOrderTimeout(order);
  }

  getRemainingTime = () => {
    const { orderDetails } = this.props;
    const timestamp = get(orderDetails, 'contract.buyerOrder.timestamp');
    const timeout = get(orderDetails, 'contract.vendorListings[0].metadata.escrowTimeoutHours');
    return timeUntil(timestamp, timeout, false);
  }

  handleCopyAddress = () => {
    const { shipping } = this.props.orderDetails.contract.buyerOrder;
    const shipTo = get(shipping, 'shipTo');
    const address = get(shipping, 'address');
    const city = get(shipping, 'city');
    const state = get(shipping, 'state');
    const postalCode = get(shipping, 'postalCode');
    const country = get(shipping, 'country');
    Clipboard.setString(`${shipTo}, ${address}, ${city}, ${state}, ${postalCode}, ${country}`);

    this.setState({ copied: true });
    setTimeout(() => this.setState({ copied: false }), 2000);
  }

  isDisputeDisabled = () => {
    const { orderType, orderDetails } = this.props;
    const { state, paymentMethod } = orderDetails;
    return (
      (state === 'PENDING' && ['ADDRESS_REQUEST', 'DIRECT'].includes(paymentMethod))
      || (orderType === 'sales' && state === 'AWAITING_FULFILLMENT')
      || this.isClosed()
    );
  }

  isClosed = () => {
    const { state } = this.props.orderDetails;
    return ['CANCELED', 'DECLINED', 'REFUNDED', 'RESOLVED', 'COMPLETED', 'PAYMENT_FINALIZED'].includes(state);
  }

  isInDispute = () => {
    const { state } = this.props.orderDetails;
    return state === 'DISPUTED' || state === 'DECIDED';
  }

  isDisputeExpired = () => {
    const { state } = this.props.orderDetails;
    return state === 'DISPUTE_EXPIRED';
  }

  isPaid = () => {
    const { paymentAddressTransactions } = this.state;
    return paymentAddressTransactions.length > 0;
  }

  handlePressDispute = () => {
    const { onDispute, orderDetails } = this.props;
    const { state } = orderDetails;
    if (this.isDisputeDisabled()) {
      switch (state) {
        case 'PENDING':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_pending_alert'));
          break;
        case 'AWAITING_FULFILLMENT':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_not_fulfilled_alert'));
          break;
        case 'CANCELED':
        case 'DECLINED':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_cancel_alert'));
          break;
        case 'REFUNDED':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_refund_alert'));
          break;
        case 'RESOLVED':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_resolved_alert'));
          break;
        case 'COMPLETED':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_completed_alert'));
          break;
        case 'PAYMENT_FINALIZED':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_finalized_alert'));
          break;
        case 'PROCESSING_ERROR':
          Alert.alert(I18n.t('components.templates.OrderSummary.oops'), I18n.t('components.templates.OrderSummary.dispute_processing_alert'));
          break;
        default:
          break;
      }
    } else {
      onDispute();
    }
  }

  handleGoToModerator = () => {
    const { navigation, orderDetails } = this.props;
    const { moderator } = orderDetails.contract.buyerOrder.payment;
    navigation.push('ModeratorDetails', { moderator });
  };

  isDisputeClosedBySeller = () => {
    const { contract } = this.props.orderDetails;
    const { vendorListings, disputeAcceptance } = contract;
    const sellerID = get(vendorListings[0], 'vendorID.peerID');
    return (sellerID === disputeAcceptance.closedBy);
  }

  renderOptions() {
    const { options, bigQuantity } = this.props.orderDetails.contract.buyerOrder.items[0];
    return (
      <View>
        {options && options.map((item, idx) => (
          <Text style={styles.option} key={`option_${idx}`}>
            {`${item.name}: ${item.value}`}
          </Text>
        ))}
        <Text style={styles.option}>
          {I18n.t('components.templates.OrderSummary.quantity_info', {quantity:bigQuantity})}
        </Text>
      </View>
    );
  }

  renderPrice = () => {
    const { localLabelFromBCH } = this.props;
    const { bigAmount, amountCurrency } = this.props.orderDetails.contract.buyerOrder.payment;
    return localLabelFromBCH(bigAmount, amountCurrency.code);
  }

  renderTransactionLinkRow = (transaction) => {
    if (!transaction) {
      return null;
    }

    const { txid, bigValue: value } = transaction;
    const { code: coinName } = this.props.orderDetails.contract.buyerOrder.payment.amountCurrency;
    const coin = COINS[coinName];
    const transactionURLs = transactionLinkDict(txid);

    return (
      <View style={styles.paymentMethod}>
        <View style={styles.paymentLabel}>
          <Image source={coin.icon} style={styles.coinImage} />
          <Text style={styles.coinName}>
            {`${getFixedCurrency(
              minUnitAmountToBCH(value || 0, coinName),
              getDecimalPoints(coinName),
            )} ${coinName}`}
          </Text>
        </View>
        <View style={{ flex: 1 }} />
        <TouchableWithoutFeedback
          onPress={() => {
            Linking.openURL(transactionURLs[coinName]);
          }}
        >
          <View>
            <Text style={styles.viewTransaction}>{(coinName === 'ETH' || coinName === 'CFX') ? I18n.t('components.templates.OrderSummary.view') : I18n.t('components.templates.OrderSummary.view_transaction')}</Text>
          </View>
        </TouchableWithoutFeedback>
      </View>
    );
  }

  renderPaymentSection = () => {
    const {
      orderType, orderDetails, onClaim,
    } = this.props;
    const { moderator } = orderDetails.contract.buyerOrder.payment;
    const { paymentAddressTransactions } = this.state;
    const transaction = paymentAddressTransactions.length > 0 ? paymentAddressTransactions[0] : null;
    const { timestamp } = transaction || {};
    const amIBuyer = orderType === 'purchases';
    const { state } = orderDetails;

    const timeSinceTimeout = this.getTimeSinceTimeout();
    const isClaimableBySeller = (this.isDisputeExpired() || timeSinceTimeout) && isFulfilled(orderDetails);
    const remaining = this.getRemainingTime();
    return (
      <InputGroup
        title={I18n.t('components.templates.OrderSummary.payment')}
        actionTitle={timestamp && timeSince(new Date(timestamp))}
        actionStyle={styles.timestamp}
      >
        {this.renderTransactionLinkRow(transaction)}
        {!this.isPaid() ? (
          <Text style={styles.unpaidNotice}>
             {I18n.t('components.templates.OrderSummary.no_payment')}            
          </Text>
        ) : isEmpty(moderator) ? (
          <Text style={styles.description}>
            {I18n.t('components.templates.OrderSummary.cannot_dispute', {user:amIBuyer ? I18n.t('common.the_seller') : I18n.t('common.you')})}
          </Text>
        ) : this.isClosed() ? (
          <Text style={styles.description}>
            {I18n.t('components.templates.OrderSummary.escrow_released')}
          </Text>
        ) : this.isInDispute() ? (
          <Text style={styles.description}>
            {I18n.t('components.templates.OrderSummary.order_in_dispute')}
            <Text style={styles.escrowDays}>{remaining}</Text>
            {I18n.t('components.templates.OrderSummary.until_accept')}
          </Text>
        ) : isClaimableBySeller ? (
          amIBuyer ? (
            <Text style={styles.description}>
              {I18n.t('components.templates.OrderSummary.period_expired_claim')}
            </Text>
          ) : (
            <Text style={styles.description}>
              {I18n.t('components.templates.OrderSummary.period_expired_claim2')}
            </Text>
          )
        ) : (
          <Text style={styles.description}>
            {I18n.t('components.templates.OrderSummary.order_in_escrow1')}
            <Text style={styles.escrowDays}>{remaining}</Text>
            {I18n.t('components.templates.OrderSummary.order_in_escrow2')}
          </Text>
        )}
        {this.isPaid() && !isEmpty(moderator) && !this.isInDispute() && !this.isDisputeExpired() && !timeSinceTimeout && (
          <TouchableWithoutFeedback onPress={this.handlePressDispute}>
            <View
              style={[
                styles.fullButton,
                this.isDisputeDisabled() && styles.disabledDisputeButton,
              ]}
            >
              <Text
                style={[
                  styles.fullText,
                  this.isDisputeDisabled() && styles.disabledDisputeText,
                ]}
              >
                {I18n.t('components.templates.OrderSummary.dispute_order')}                
              </Text>
            </View>
          </TouchableWithoutFeedback>
        )}
        {orderType === 'sales' && isClaimableBySeller && state !== 'PAYMENT_FINALIZED' && (
          <TouchableWithoutFeedback onPress={onClaim}>
            <View style={styles.claimButton}>
              <Text style={styles.claimText}>
                {I18n.t('components.templates.OrderSummary.claim_payment')}
              </Text>
            </View>
          </TouchableWithoutFeedback>)}
        {orderType === 'sales' && isClaimableBySeller && state === 'PAYMENT_FINALIZED' && (
        <TouchableWithoutFeedback>
          <View
            style={[
              styles.fullButton, styles.disabledDisputeButton,
            ]}
          >
            <Text
              style={[
                styles.fullText, styles.disabledDisputeText,
              ]}
            >
              {I18n.t('components.templates.OrderSummary.dispute_order')}              
            </Text>
          </View>
        </TouchableWithoutFeedback>
        )}
      </InputGroup>
    );
  }

  renderErrorSection = () => {
    const { orderDetails, orderType } = this.props;
    const { state, contract } = orderDetails;
    const { moderator } = contract.buyerOrder.payment;

    if (state !== 'PROCESSING_ERROR') {
      return null;
    }

    const { paymentAddressTransactions } = this.state;
    if (paymentAddressTransactions.length === 0) {
      return null;
    }
    const timestamp = get(paymentAddressTransactions[0], 'timestamp');

    const disputePossible = (
      orderType === 'purchases' && !isEmpty(moderator))
      || (orderType === 'sales' && isFulfilled(orderDetails)
      );
    return (
      <InputGroup
        title={[<Ionicons name="md-alert" color={warningColor} size={18} />, ' Error']}
        actionTitle={this.isPaid() && timeSince(new Date(timestamp))}
        actionStyle={styles.timestamp}
      >
        <Text style={styles.description}>
          {disputePossible ? (
            I18n.t('components.templates.OrderSummary.dispute_error_possible')         
          ) : (
            I18n.t('components.templates.OrderSummary.dispute_error')
          )}
        </Text>
      </InputGroup>
    );
  }

  renderClosedSection = () => {
    if (!this.isClosed()) {
      return null;
    }

    const { orderType, orderDetails } = this.props;
    const { contract, state, refundAddressTransaction } = orderDetails;
    const {
      buyerOrder, buyerOrderCompletion, dispute, vendorOrderConfirmation, disputeResolution,
    } = contract;
    const { buyerID, payment } = buyerOrder;
    const paymentMethod = get(orderDetails, 'contract.buyerOrder.payment.method');
    const { peerID } = buyerID;
    const { paymentAddressTransactions } = this.state;
    const rating = get(buyerOrderCompletion, 'ratings[0]');

    const amIBuyer = orderType === 'purchases';
    const buyerValue = get(disputeResolution, 'payout.buyerOutput.amount');
    const sellerValue = get(disputeResolution, 'payout.vendorOutput.amount');

    let title, transaction, timestamp, description;
    if (state === 'REFUNDED') {
      title = I18n.t('components.templates.OrderSummary.order_refunded') ;
      transaction = refundAddressTransaction;
      timestamp = (transaction || {}).timestamp;
      description = I18n.t('components.templates.OrderSummary.full_refund');
    } else {
      transaction = paymentAddressTransactions[1];
      timestamp = (transaction || {}).timestamp;
      if (state === 'COMPLETED') {
        title = I18n.t('components.templates.OrderSummary.order_completed');

        description = transaction ?  I18n.t('components.templates.OrderSummary.release_to_seller') : null;
        // timestamp = get(buyerOrderCompletion, 'timestamp');
        if (!isEmpty(dispute)) {
          transaction = null;
          description = null;
        } else {
          transaction = {
            ...transaction,
            // bigValue: get(vendorOrderConfirmation, 'bigRequestedAmount'),
          };
        }
      } else if (state === 'RESOLVED') {
        title = I18n.t('components.templates.OrderSummary.dispute_closed');
        description = I18n.t('components.templates.OrderSummary.dispute_closed_info', {user: this.isDisputeClosedBySeller() ?I18n.t('components.templates.OrderSummary.the_seller'):I18n.t('components.templates.OrderSummary.the_buyer')});
        transaction = {
          ...transaction,
          // bigValue: amIBuyer ? buyerValue : sellerValue,
        };
      } else if (state === 'PAYMENT_FINALIZED') {
        title = I18n.t('components.templates.OrderSummary.payment_claimed');
        description = I18n.t('components.templates.OrderSummary.seller_claim');

        transaction = {
          ...transaction,
          // bigValue: get(vendorOrderConfirmation, 'bigRequestedAmount'),
        };
      } else {
        title = I18n.t('components.templates.OrderSummary.order_canceled');
        description = I18n.t('components.templates.OrderSummary.user_cancel_order');

        transaction = {
          ...transaction,
          // bigValue: get(payment, 'bigAmount'),
        };
      }
    }

    const shouldRenderRatingGroup = state === 'COMPLETED' && !isEmpty(rating) && transaction;
    const { code: coinName } = this.props.orderDetails.contract.buyerOrder.payment.amountCurrency;

    if (state === 'COMPLETED' && ['ADDRESS_REQUEST', 'DIRECT'].includes(paymentMethod)) {
      return ([
        <InputGroup
          title={title}
          actionTitle={this.isPaid() && timestamp && timeSince(new Date(timestamp))}
          actionStyle={styles.timestamp}
          wrapperStyle={shouldRenderRatingGroup && styles.paddedBottom}
          noPadding={shouldRenderRatingGroup}
          noHeaderPadding={shouldRenderRatingGroup}
          noBorder
        >
          {state === 'COMPLETED' && !isEmpty(rating) && !transaction && (
            <OrderRating noTopPadding rating={rating} buyerID={peerID} />
          )}
        </InputGroup>,
        shouldRenderRatingGroup && (
          <InputGroup noHeaderTopPadding>
            <OrderRating noTopPadding rating={rating} buyerID={peerID} />
          </InputGroup>
        ),
      ]);
    } else {
      return ([
        <InputGroup
          title={title}
          actionTitle={this.isPaid() && timestamp && timeSince(new Date(timestamp))}
          actionStyle={styles.timestamp}
          wrapperStyle={shouldRenderRatingGroup && styles.paddedBottom}
          noPadding={shouldRenderRatingGroup}
          noHeaderPadding={shouldRenderRatingGroup}
        >
          {coinName !== 'ETH' && this.renderTransactionLinkRow(transaction)}
          {description && <Text style={styles.description}>{description}</Text>}
          {state === 'COMPLETED' && !isEmpty(rating) && !transaction && (
            <OrderRating rating={rating} buyerID={peerID} />
          )}
        </InputGroup>,
        shouldRenderRatingGroup && (
          <InputGroup>
            <OrderRating noTopPadding rating={rating} buyerID={peerID} />
          </InputGroup>
        ),
      ]);
    }
  }

  renderShippingAddress = (shipping) => {
    const {
      shipTo = '',
      address = '',
      city = '',
      state = '',
      postalCode = '',
      country = '',
      addressNotes,
    } = shipping;
    return (
      <View style={styles.addrWrapper}>
        <Text style={styles.addrName}>{shipTo}</Text>
        <Text style={styles.addrLine}>{address}</Text>
        <Text style={styles.addrLine}>
          {`${city}, ${state} ${postalCode}`}
        </Text>
        <Text style={styles.addrLine}>
          {parseCountryName(country)}
        </Text>
        {!isEmpty(addressNotes) && (
          <Text style={styles.addrNote}>{decode(addressNotes)}</Text>
        )}
      </View>
    );
  }

  renderOrderTimeout = () => {
    const timeSinceTimeout = this.getTimeSinceTimeout();
    const { orderDetails } = this.props;

    const { state } = orderDetails;
    if (!timeSinceTimeout || state === 'DISPUTE_EXPIRED' || orderDetails.contract.hasOwnProperty('dispute') === true) {
      return null;
    }

    return (
      <InputGroup
        title={I18n.t('components.templates.OrderSummary.period_expired')}
        actionTitle={timeSinceTimeout}
        actionStyle={styles.timestamp}
      >
        <Text style={styles.description}>
          {I18n.t('components.templates.OrderSummary.no_dispute')}          
        </Text>
      </InputGroup>
    );
  }

  render() {
    const {
      localLabelFromBCH,
      onMessage,
      orderDetails = {},
      orderId,
      orderType,
    } = this.props;

    const { vendorListings = [], buyerOrder } = orderDetails.contract || {};
    if (!vendorListings[0]) {
      return null;
    }

    const { items, shipping, payment } = buyerOrder || {};
    const { amountCurrency, moderator, bigAmount } = payment;
    const { code: coinName } = amountCurrency;

    const { copied } = this.state;
    const { bigQuantity, memo } = items[0];
    const { item, metadata } = vendorListings[0];
    const { title, priceCurrency = [] } = item;
    const { contractType } = metadata;
    const dispute = get(orderDetails, 'contract.dispute');
    const vendorID = get(vendorListings[0], 'vendorID.peerID');
    const vendorOrderFulfillment = get(orderDetails, 'contract.vendorOrderFulfillment');
    const shippingDetails = contractType === 'PHYSICAL_GOOD' ? this.getShippingDetails() : undefined;
    const price = BigNumber(bigAmount).minus(BigNumber(shippingDetails ? shippingDetails.bigPrice : 0)).div(BigNumber(bigQuantity));
    return (
      <View>
        <InputGroup title={I18n.t('components.templates.OrderSummary.Summary')}>
          <ListingTitlePrice
            title={decode(title)}
            price={price}
            currency={coinName}
            quantity={bigQuantity}
          />
          {this.renderOptions()}
          {contractType === 'PHYSICAL_GOOD' && (
            <View>
              {isEmpty(shippingDetails) ? (
                <View style={styles.optionWrapper}>
                  <Text style={styles.optionPlaceholder}> {I18n.t('components.templates.OrderSummary.Shipping')}</Text>
                </View>
              ) : (
                <View style={styles.optionWrapper}>
                  <Text style={styles.optionLabel}>
                    {`${shippingDetails.optionName} ${shippingDetails.name}  `}
                    <Text style={styles.estDelivery}>{shippingDetails.estimatedDelivery}</Text>
                  </Text>
                  <Text style={styles.price}>
                    {isEmpty(shippingDetails.bigPrice) ? (
                      'FREE'
                    ) : (
                      localLabelFromBCH(shippingDetails.bigPrice, priceCurrency.code)
                    )}
                  </Text>
                </View>
                )}
            </View>
          )}
          <Dash dashColor={borderColor} dashGap={2} dashLength={8} dashThickness={1} />
          <View style={styles.totalWrapper}>
            <Text style={styles.totalLabel}> {I18n.t('components.templates.OrderSummary.total')}</Text>
            <Text style={styles.totalValue}>{this.renderPrice()}</Text>
          </View>
        </InputGroup>
        {this.renderClosedSection()}
        {this.renderErrorSection()}
        {this.renderOrderTimeout()}
        {!isEmpty(dispute) && (
          <OrderDispute
            vendorId={vendorID}
            orderId={orderId}
            orderDetails={orderDetails}
            onMessage={onMessage}
            orderType={orderType}
          />
        )}
        {!isEmpty(vendorOrderFulfillment) && (
          <OrderFulfillment peerID={vendorID} detail={vendorOrderFulfillment[0]} />
        )}
        {this.renderPaymentSection()}
        {!isEmpty(moderator) && (
          <TouchableWithoutFeedback onPress={this.handleGoToModerator}>
            <View style={styles.moderator}>
              <Moderator peerID={moderator} />
            </View>
          </TouchableWithoutFeedback>
        )}
        {contractType === 'PHYSICAL_GOOD' && (
          <InputGroup
            title={I18n.t('components.templates.OrderSummary.Shipping')}
            actionTitle={<Ionicons name="md-copy" color={brandColor} size={24} />}
            action={this.handleCopyAddress}
          >
            {!isEmpty(shipping) && this.renderShippingAddress(shipping)}
          </InputGroup>
        )}
        <InputGroup title={I18n.t('components.templates.OrderSummary.Note')} noBorder>
          {isEmpty(memo) ? (
            <Text style={styles.noMemoStyle}>{I18n.t('components.templates.OrderSummary.no_buyer_note')}</Text>
          ) : (
            <Text style={styles.memoText}>{memo}</Text>
          )}
        </InputGroup>
        {copied && (
          <View style={styles.overlay}>
            <Text style={styles.overlayText}>{I18n.t('components.templates.OrderSummary.address_copied')}</Text>
          </View>
        )}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default connect(mapStateToProps)(OrderSummary);
