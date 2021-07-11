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
      return { name: disputerName, text: I18n.t('utils.notification.started_disputed') };
    case 'disputeClose':
      return { name: moderatorName, text: I18n.t('utils.notification.proposed_dispute_outcome') };
    case 'disputeAccepted':
      if (buyerAccepted) {
        return ({ name: buyerName, text: I18n.t('utils.notification.accepted_dispute_payout') });
      }
      return ({ name: vendorName, text: I18n.t('utils.notification.accepted_dispute_payout') });
    case 'vendorFinalizePayment':
      return ({ name: vendorName, text: I18n.t('utils.notification.claimed_their_payment') });
    case 'buyerDisputeExpiry': {
      const days = getDays(expiresIn);
      const daysLeft = (days === 1) ? I18n.t('utils.notification.day') : I18n.t('utils.notification.days');
      return ({ name: moderatorName, text: I18n.t('utils.notification.has_left', {days: days, daysLeft:daysLeft}) });
    }
    default:
      return { name: disputeeName, text: ` ${type}` };
  }
};

export const getOrderText = (type, buyer, vendor, isBuyer) => {
  switch (type) {
    case 'order':
      return isBuyer ? { name: '', text: I18n.t('utils.notification.you_placed_order') } : { name: buyer, text: I18n.t('utils.notification.placed_order') };
    case 'payment':
      return isBuyer ? { name: '', text: I18n.t('utils.notification.your_payment_sent') } : { name: buyer, text: I18n.t('utils.notification.sent_payment') };
    case 'orderDeclined':
      return isBuyer ? { name: vendor, text: I18n.t('utils.notification.cancelled_your_order') } : { name: I18n.t('utils.notification.you'), text: I18n.t('utils.notification.declined_order') };
    case 'orderConfirmation':
      return isBuyer ? { name: vendor, text: I18n.t('utils.notification.accepted_your_order') } : { name: I18n.t('utils.notification.you'), text: I18n.t('utils.notification.accepted_order') };
    case 'cancel':
      return isBuyer ? { name: I18n.t('utils.notification.you'), text: I18n.t('utils.notification.cancelled_this_order') } : { name: buyer, text: I18n.t('utils.notification.cancelled_their_order') };
    case 'refund':
      return isBuyer ? { name: vendor, text: I18n.t('utils.notification.refunded_your_order') } : { name: I18n.t('utils.notification.you'), text: I18n.t('utils.notification.refunded_this_order') };
    case 'fulfillment':
      return isBuyer ? { name: vendor, text: I18n.t('utils.notification.fulfilled_your_order') } : { name: I18n.t('utils.notification.you'), text: I18n.t('utils.notification.fulfilled_order') };
    case 'orderComplete':
      return { name: buyer, text: I18n.t('utils.notification.completed_their_order') };
    default:
      return { name: vendor, text: ` ${type}` };
  }
};
