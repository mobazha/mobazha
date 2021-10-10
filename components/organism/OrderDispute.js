import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Alert, View, Text, TouchableWithoutFeedback } from 'react-native';
import decode from 'unescape';
import { get, isEmpty } from 'lodash';

import InputGroup from '../atoms/InputGroup';

import { getProfile } from '../../api/profile';
import { convertorsMap } from '../../selectors/currency';
import { borderColor, foregroundColor, primaryTextColor, brandColor, formLabelColor } from '../commonColors';
import { timeSince } from '../../utils/time';
import { EXPIRE_IN_HOURS } from '../../utils/order';

import { acceptPayout } from '../../reducers/order';

import {I18n} from '../../langs/I18n';

const styles = {
  acceptButton: {
    width: '100%',
    paddingHorizontal: 17,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: brandColor,
    justifyContent: 'center',
    marginTop: 14,
    marginBottom: 18,
  },
  acceptText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: 'white',
  },
  memoContainer: {
    paddingLeft: 9,
    borderLeftWidth: 2,
    borderColor,
    marginTop: 13,
  },
  memo: {
    fontSize: 15,
    color: primaryTextColor,
  },
  memoComment: {
    marginTop: 13,
    fontSize: 15,
    color: primaryTextColor,
  },
  serviceFee: {
    marginTop: 8,
    marginBottom: 8,
    fontSize: 13,
    lineHeight: 13,
    color: formLabelColor,
  },
  messageButton: {
    width: '100%',
    paddingHorizontal: 17,
    paddingVertical: 11,
    borderRadius: 2,
    backgroundColor: foregroundColor,
    borderStyle: 'solid',
    borderWidth: 1,
    borderColor: '#c8c7cc',
    justifyContent: 'center',
    marginTop: 14,
    marginBottom: 18,
  },
  messageText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'center',
    color: primaryTextColor,
  },
  timestamp: {
    color: '#9b9b9b',
  },
  payoutMainAmount: {
    fontWeight: 'bold',
    color: brandColor,
  },
  payoutAmount: {
    fontWeight: 'bold',
  },
  spacer: {
    marginBottom: 18,
  },
};

class OrderDispute extends PureComponent {
  state = {
    sellerStarted: false,
    buyerProfile: {},
    sellerProfile: {},
  };

  UNSAFE_componentWillMount() {
    const { orderDetails } = this.props;
    const dispute = get(orderDetails, 'contract.dispute');
    if (!isEmpty(dispute)) {
      const vendorOrderFulfillment = get(orderDetails, 'contract.vendorOrderFulfillment');
      const sellerPayout = !isEmpty(vendorOrderFulfillment) && get(vendorOrderFulfillment[0], 'payout.payoutAddress');
      const disputePayoutAddr = get(dispute, 'payoutAddress');
      const peerID = get(orderDetails, 'contract.buyerOrder.buyerID.peerID');
      const vendorListings = get(orderDetails, 'contract.vendorListings');
      const sellerID = get(vendorListings[0], 'vendorID.peerID');
      if (sellerPayout === disputePayoutAddr) {
        this.setState({
          sellerStarted: true,
        });
      }
      getProfile(sellerID).then((response) => {
        this.setState({ sellerProfile: response });
      });
      getProfile(peerID).then((response) => {
        this.setState({ buyerProfile: response });
      });
    }
  }

  handleAcceptPayout = () => {
    const { acceptPayout, orderDetails, orderId, orderType, vendorId } = this.props;
    const buyerId = get(orderDetails, 'contract.buyerOrder.buyerID.peerID');
    Alert.alert(
      'Accept payout?',
      'Once accepted, the dispute will close and the funds will transfer',
      [
        { text: 'Cancel' },
        {
          text: 'Ok',
          onPress: () => {
            acceptPayout({ orderId, orderType, vendorId, buyerId });
          },
        },
      ],
    );
  }

  renderDisputeExpire() {
    const { orderDetails } = this.props;

    const { state } = orderDetails;
    if (!['DISPUTE_EXPIRED', 'PAYMENT_FINALIZED'].includes(state)) {
      return null;
    }

    const disputeTimestamp = get(orderDetails, 'contract.dispute.timestamp');

    if (!disputeTimestamp) {
      return null;
    }

    const disputeStartDay = new Date(disputeTimestamp);
    const timeoutDay = disputeStartDay.setHours(disputeStartDay.getHours() + EXPIRE_IN_HOURS);

    return (
      <InputGroup
        title="Dispute expired"
        actionTitle={timeSince(new Date(timeoutDay))}
        actionStyle={styles.timestamp}
      >
        <Text style={styles.memoComment}>
          {I18n.t('components.organism.OrderDispute.memo_comment1')}
        </Text>
        <View style={styles.spacer} />
      </InputGroup>
    );
  }

  renderDisputePayout() {
    const { orderDetails, localLabelFromBCH, orderType } = this.props;
    const { contract, state } = orderDetails;
    const { buyerOrder, disputeResolution, disputeAcceptance } = contract;
    const buyerValue = get(disputeResolution, 'payout.buyerOutput.bigAmount');
    const sellerValue = get(disputeResolution, 'payout.vendorOutput.bigAmount');
    const moderatorValue = get(disputeResolution, 'payout.moderatorOutput.bigAmount');
    const memo = get(disputeResolution, 'resolution');
    const coin = get(buyerOrder, 'payment.amountCurrency.code');
    const amIBuyer = orderType === 'purchases';
    return (
      <InputGroup
        title={I18n.t('components.organism.OrderDispute.dispute_payout')}
        actionTitle={timeSince(new Date(disputeResolution.timestamp))}
        actionStyle={styles.timestamp}
      >
        {!isEmpty(memo) && (
          <View style={styles.memoContainer}>
            <Text style={styles.memo}>{decode(memo)}</Text>
          </View>
        )}
        <Text style={styles.memoComment}>
          <Text style={styles.payoutMainAmount}>
            {localLabelFromBCH((amIBuyer ? buyerValue : sellerValue) || 0, coin)}
          </Text>
          {I18n.t('components.organism.OrderDispute.will_be_issued')}
        </Text>
        <Text style={[styles.serviceFee, state !== 'DECIDED' && { marginBottom: 18 }]}>
          {I18n.t('components.organism.OrderDispute.moderator_takes')}
          <Text style={styles.payoutAmount}>{localLabelFromBCH(moderatorValue || 0, coin)}</Text>
          {'. '}
          {amIBuyer ? I18n.t('components.organism.OrderDispute.seller_takes') : I18n.t('components.organism.OrderDispute.buyer_takes')}
          <Text style={styles.payoutAmount}>
            {localLabelFromBCH((amIBuyer ? sellerValue : buyerValue) || 0, coin)}
          </Text>
          .
        </Text>
        {state === 'DECIDED' && !disputeAcceptance && (
          <TouchableWithoutFeedback onPress={this.handleAcceptPayout}>
            <View style={styles.acceptButton} >
              <Text style={styles.acceptText}>{I18n.t('components.organism.OrderDispute.accept_payout')}</Text>
            </View>
          </TouchableWithoutFeedback>
        )}
      </InputGroup>
    );
  }

  render() {
    const { orderDetails, onMessage } = this.props;
    const dispute = get(orderDetails, 'contract.dispute');
    if (isEmpty(dispute)) {
      return null;
    }

    const disputeResolution = get(orderDetails, 'contract.disputeResolution');
    const {
      buyerProfile, sellerProfile, sellerStarted,
    } = this.state;
    const profile = sellerStarted ? sellerProfile : buyerProfile;
    const name = get(profile, 'name');
    return (
      <View>
        {this.renderDisputeExpire()}
        {!isEmpty(disputeResolution) && this.renderDisputePayout()}
        <InputGroup
          title={I18n.t('components.organism.OrderDispute.started_by', {name:name}) + " " + (sellerStarted ? I18n.t('components.organism.OrderDispute.seller_takes') : I18n.t('components.organism.OrderDispute.buyer_takes'))}
          actionTitle={timeSince(new Date(dispute.timestamp))}
          actionStyle={styles.timestamp}
        >
          <View style={styles.memoContainer}>
            <Text style={styles.memo}>
              {decode(dispute.claim)}
            </Text>
          </View>
          <Text style={styles.memoComment}>
            {I18n.t('components.organism.OrderDispute.memo_comment2')}
          </Text>
          <TouchableWithoutFeedback onPress={onMessage}>
            <View style={styles.messageButton}>
              <Text style={styles.messageText}>{I18n.t('components.organism.OrderDispute.message')}</Text>
            </View>
          </TouchableWithoutFeedback>
        </InputGroup>
      </View>
    );
  }
}

const mapStateToProps = state => ({
  username: state.appstate.username,
  password: state.appstate.password,
  ...convertorsMap(state),
});

const mapDispatchToProps = {
  acceptPayout,
};

export default connect(mapStateToProps, mapDispatchToProps)(OrderDispute);
