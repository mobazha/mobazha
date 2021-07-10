import { findIndex, hasIn } from 'lodash';
import {I18n} from '../langs/I18n';

export const orderNotificationTypes = [
  'order',
  'payment',
  'orderConfirmation',
  'cancel',
  'refund',
  'fulfillment',
  'orderComplete',
  'orderDeclined',
];

export const disputeNotificationTypes = [
  'disputeOpen',
  'disputeClose',
  'disputeAccepted',
  'buyerDisputeExpiry',
  'vendorFinalizePayment',
];

export const peerNotificationTypes = ['follow', 'unfollow', 'moderatorAdd', 'moderatorRemove'];
export const SOCIAL_TYPES = ['comment', 'like', 'repost', 'follow', 'unfollow'];

export const getNotificationType = (type) => {
  const order = findIndex(orderNotificationTypes, o => o === type);
  if (order >= 0) {
    return 'order';
  }
  const peer = findIndex(peerNotificationTypes, o => o === type);
  if (peer >= 0) {
    return 'peer';
  }
  const dispute = findIndex(disputeNotificationTypes, o => o === type);
  if (dispute >= 0) {
    return 'dispute';
  }
  return 'unknown';
};

export const filterOrderNotification = notifications =>
  notifications.filter(({ type }) => getNotificationType(type) !== 'peer');

export const filterNotifications = (notifications, type) =>
  notifications.filter((notification) => {
    if (hasIn(notification, 'group') && type === 'social' && SOCIAL_TYPES.includes(notification.verb)) {
      return true;
    }
    if (hasIn(notification, 'type') && getNotificationType(notification.type) === type) {
      return true;
    }
    return false;
  });

const getDays = expiresIn => Math.ceil(expiresIn / 3600 / 24);

export const getDisputeText = (
  type,
  disputeeName,
  disputerName,
  otherParty,
  buyerName,
  vendorName,
  moderatorName,
  buyerAccepted,
  expiresIn,
) => {
  switch (type) {
    case 'disputeOpen':
      return { name: disputerName, text: ' started a dispute' };
    case 'disputeClose':
      return { name: moderatorName, text: ' proposed a dispute outcome' };
    case 'disputeAccepted':
      if (buyerAccepted) {
        return ({ name: buyerName, text: ' accepted dispute payout' });
      }
      return ({ name: vendorName, text: ' accepted dispute payout' });
    case 'vendorFinalizePayment':
      return ({ name: vendorName, text: ' claimed their payment' });
    case 'buyerDisputeExpiry': {
      const days = getDays(expiresIn);
      const daysLeft = (days === 1) ? 'day' : 'days';
      return ({ name: moderatorName, text: ` has ${days} ${daysLeft} left to propose a dispute outcome` });
    }
    default:
      return { name: disputeeName, text: ` ${type}` };
  }
};

export const getOrderText = (type, buyer, vendor, isBuyer) => {
  switch (type) {
    case 'order':
      return isBuyer ? { name: '', text: 'You placed an order' } : { name: buyer, text: ' placed an order' };
    case 'payment':
      return isBuyer ? { name: '', text: 'Your payment has been sent' } : { name: buyer, text: ' sent their payment' };
    case 'orderDeclined':
      return isBuyer ? { name: vendor, text: ' cancelled your order' } : { name: 'You', text: ' declined this order' };
    case 'orderConfirmation':
      return isBuyer ? { name: vendor, text: ' accepted your order' } : { name: 'You', text: ' accepted this order' };
    case 'cancel':
      return isBuyer ? { name: 'You', text: ' cancelled this order' } : { name: buyer, text: ' cancelled their order' };
    case 'refund':
      return isBuyer ? { name: vendor, text: ' refunded your order' } : { name: 'You ', text: ' refunded this order' };
    case 'fulfillment':
      return isBuyer ? { name: vendor, text: ' fulfilled your order' } : { name: 'You ', text: ' fulfilled this order' };
    case 'orderComplete':
      return { name: buyer, text: ' completed their order' };
    default:
      return { name: vendor, text: ` ${type}` };
  }
};
