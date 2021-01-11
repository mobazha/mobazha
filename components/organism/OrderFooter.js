import React, { PureComponent } from 'react';
import { connect } from 'react-redux';
import { Image, Text, TouchableOpacity, View } from 'react-native';
import { withNavigation } from 'react-navigation';
import * as _ from 'lodash';
import Ionicons from 'react-native-vector-icons/Ionicons';
import { ifIphoneX } from 'react-native-iphone-x-helper';

import { primaryTextColor, borderColor, formLabelColor, brandColor, warningColor, starRatingColor } from '../commonColors';
import { convertorsMap } from '../../selectors/currency';
import { COINS } from '../../utils/coins';
import { timeUntil } from '../../utils/time';
import { getOrderActions, getStatusInfo, isFulfilled, timeSinceOrderTimeout } from '../../utils/order';
import { minUnitAmountToBCH } from '../../utils/currency';

import {I18n} from '../../langs/I18n';

const styles = {
  wrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    height: 70,
    borderTopWidth: 1,
    borderColor,
    backgroundColor: 'white',
    paddingHorizontal: 17,
    marginBottom: ifIphoneX(34, 0),
  },
  buttonWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-end',
  },
  button: {
    marginLeft: 6,
    paddingVertical: 8,
    paddingHorizontal: 22,
    borderRadius: 2,
    borderWidth: 1,
    borderColor: '#c8c7cc',
  },
  buttonText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: primaryTextColor,
  },
  mainButton: {
    marginLeft: 6,
    paddingVertical: 8,
    paddingHorizontal: 22,
    borderRadius: 2,
    backgroundColor: brandColor,
    borderColor: brandColor,
    borderWidth: 1,
  },
  mainButtonText: {
    fontSize: 13,
    fontWeight: 'bold',
    fontStyle: 'normal',
    letterSpacing: 0,
    color: 'white',
  },
  statusRed: {
    fontSize: 14,
    color: warningColor,
  },
  status: {
    fontSize: 14,
    color: primaryTextColor,
  },
  statusBrand: {
    fontSize: 14,
    color: brandColor,
  },
  statusBrandBold: {
    fontSize: 14,
    fontWeight: 'bold',
    color: brandColor,
  },
  disputeTimestamp: {
    fontSize: 14,
    alignItems: 'center',
    fontWeight: 'bold',
    color: warningColor,
  },
  priceWrapper: {
    alignSelf: 'center',
  },
  localPrice: {
    fontSize: 18,
    lineHeight: 18,
    fontWeight: 'bold',
    fontStyle: 'normal',
    textAlign: 'left',
    marginBottom: 2,
    color: brandColor,
  },
  cryptoWrapper: {
    marginTop: 3,
    flexDirection: 'row',
  },
  icon: {
    width: 15,
    height: 15,
  },
  crypto: {
    marginLeft: 3,
    fontSize: 13,
    color: formLabelColor,
  },
  avgRating: {
    flexDirection: 'row',
    alignItems: 'center',
  },
};

class OrderFooter extends PureComponent {
  renderLastAction = (action) => {
    const {
      order: { total },
      onOrderAction,
    } = this.props;

    const showMainStyle = !['Dispute', 'Cancel', 'Write a review'].includes(action);
    return (
      <TouchableOpacity
        style={showMainStyle ? styles.mainButton : styles.button}
        onPress={() => {
          onOrderAction(action, total);
        }}
      >
        <Text style={showMainStyle ? styles.mainButtonText : styles.buttonText}>
          {action}
        </Text>
      </TouchableOpacity>
    );
  }

  renderActionButtons() {
    const {
      order,
      orderDetails, // used in DISPUTED state only
      orderType,
      onOrderAction,
      localLabelFromBCH,
    } = this.props;

    const {
      state, total, paymentCoin,
    } = order;

    if (state === 'DISPUTED') {
      const timestamp = _.get(orderDetails, 'contract.dispute.timestamp');
      const timeout = _.get(orderDetails, 'contract.vendorListings[0].metadata.escrowTimeoutHours');

      if (!timestamp) {
        return null;
      }

      return (
        <Text style={styles.disputeTimestamp}>
          <Ionicons name="md-alarm" size={18} color={warningColor} />
          {` ${timeUntil(timestamp, timeout)}`}
        </Text>
      );
    }

    if (state === 'COMPLETED') {
      const avgRating = _.get(orderDetails, 'contract.buyerOrderCompletion.ratings[0].ratingData.overall');
      return avgRating && (
        <View style={styles.avgRating}>
          <Ionicons name="md-star" size={22} color={starRatingColor} />
          <Text style={styles.status}>{` ${avgRating.toFixed(1)} `}</Text>
        </View>
      );
    }

    if (state === 'DECIDED') {
      const buyerValue = _.get(orderDetails, 'contract.disputeResolution.payout.buyerOutput.bigAmount');
      const sellerValue = _.get(orderDetails, 'contract.disputeResolution.payout.vendorOutput.bigAmount');
      return (
        <Text style={styles.statusBrandBold}>
          {localLabelFromBCH((orderType === 'purchases' ? buyerValue : sellerValue) || 0, paymentCoin)}
        </Text>
      );
    }

    if (state === 'RESOLVED' && !isFulfilled(orderDetails)) {
      return null;
    }

    const actions = getOrderActions(orderType, order, orderDetails);
    if (!actions || actions.length === 0) {
      return null;
    }

    if (actions.length === 1) {
      return (
        <View style={styles.buttonWrapper}>{this.renderLastAction(actions[0])}</View>
      );
    } else {
      const subActions = actions.splice(0, actions.length - 1);
      return (
        <View style={styles.buttonWrapper}>
          {subActions.map((val, idx) => (
            <TouchableOpacity
              key={`subActionBtn_${idx}`}
              style={styles.button}
              onPress={() => {
                onOrderAction(val, total);
              }}
            >
              <Text style={styles.buttonText}>{val}</Text>
            </TouchableOpacity>
          ))}
          {this.renderLastAction(actions[actions.length - 1])}
        </View>
      );
    }
  }

  renderStatus = () => {
    const { orderType, localLabelFromBCH, order } = this.props;
    const { total, paymentCoin, state } = order;

    const info = getStatusInfo(state, orderType);
    const label = info ? info.label : state;
    if (state === 'AWAITING_PAYMENT') {
      return (
        <View style={styles.priceWrapper}>
          <Text style={[styles.localPrice]}>
            {localLabelFromBCH(total.amount, paymentCoin)}
          </Text>
          <View style={styles.cryptoWrapper}>
            <Image style={styles.icon} source={COINS[paymentCoin].icon} resizeMode="cover" />
            <Text style={styles.crypto}>
              {`${minUnitAmountToBCH(total.amount, paymentCoin)} ${paymentCoin}`}
            </Text>
          </View>
        </View>
      );
    } else if (['COMPLETED'].includes(state)) {
      return (
        <Text style={styles.statusBrand}>{label}</Text>
      );
    } else if (['CANCELED', 'DECLINED', 'DISPUTED', 'REFUNDED', 'PROCESSING_ERROR'].includes(state)) {
      return (
        <Text style={styles.statusRed}>{label}</Text>
      );
    } else if (state === 'DISPUTE_EXPIRED' || (state === 'FULFILLED' && timeSinceOrderTimeout(order))) {
      if (orderType === 'purchases') {
        return (
          <Text style={state === 'DISPUTE_EXPIRED' ? styles.statusRed : styles.status}>{label}</Text>
        );
      } else {
        return (
          <Text style={styles.status}>
            {I18n.t('components.organism.OrderFooter.claim')}
            <Text style={styles.statusBrandBold}>{localLabelFromBCH(total.amount, paymentCoin)}</Text>
            ?
          </Text>
        );
      }
    } else if (['FULFILLED', 'PENDING', 'AWAITING_FULFILLMENT'].includes(state)) {
      return (
        <Text style={styles.status}>{label}</Text>
      );
    }

    return (
      <Text style={styles.status}>{label}</Text>
    );
  }

  render() {
    return (
      <View style={styles.wrapper}>
        {this.renderStatus()}
        <View style={{ flex: 1 }} />
        {this.renderActionButtons()}
      </View>
    );
  }
}

const mapStateToProps = state => ({
  ...convertorsMap(state),
});

export default withNavigation(connect(mapStateToProps)(OrderFooter));
