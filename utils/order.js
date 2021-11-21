/* eslint-disable prefer-spread */
import * as _ from 'lodash';
import orderStatusEn from '../config/orderStatus.json';
import orderStatusZh from '../config/zh/orderStatus.json';
import { timeSinceInSeconds, formatSeconds } from './time.js';

import {I18n} from '../langs/I18n';

let orderStatus = orderStatusEn
if (I18n.locale.startsWith('zh')) {
  orderStatus = orderStatusZh
}

export const EXPIRE_IN_HOURS = 45 * 24;
// export const EXPIRE_IN_HOURS = 1;

export const isOrderStatusInCategory = (val, category) => {
  const valueList = orderStatus[category].items.map(item => item.value);
  return valueList.includes(val);
};

export const getStatusInfo = (val, orderType) => {
  const flattenedItems = [].concat.apply(
    [],
    Object.values(orderStatus)
      .filter(category => category.orderType.includes(orderType))
      .map(category => category.items),
  );
  return flattenedItems.find(item => item.value === val);
};

export const isFulfilled = (orderDetails) => {
  const vendorOrderFulfillment = _.get(orderDetails, 'contract.vendorOrderFulfillment');
  if (_.isEmpty(vendorOrderFulfillment)) {
    return false;
  }

  return true;
};

export const timeSinceOrderTimeout = (order) => {
  const { state, timestamp, moderated } = order;
  if (!moderated) {
    return null;
  }

  const categoryFilter = ['Awaiting payment', 'In dispute', 'Closed', 'Error'];
  const flattenedItems = [].concat.apply(
    [],
    Object.keys(orderStatus)
      .filter(category => categoryFilter.includes(category))
      .map(category => orderStatus[category].items),
  );

  if (state !== 'PAYMENT_FINALIZED' && flattenedItems.find(item => item.value === state)) {
    return null;
  }

  const timeDiff = timeSinceInSeconds(new Date(timestamp)) - (EXPIRE_IN_HOURS * 3600);
  if (timeDiff > 0) {
    return formatSeconds(timeDiff);
  }

  return null;
};

export const getOrderActions = (orderType, order, orderDetails) => {
  const { state, moderated } = order;
  const info = getStatusInfo(state, orderType);
  if (!info) {
    return null;
  }

  let actions = info.actions[orderType];
  if (!actions) {
    return [];
  }

  actions = actions.filter((action) => {
    if (action === 'Dispute') {
      if (moderated) {
        if (orderType === 'purchases') {
          if (timeSinceOrderTimeout(order) && ['AWAITING_FULFILLMENT', 'FULFILLED'].includes(state)) {
            return false;
          } else {
            return true;
          }
        } else {
          return isFulfilled(orderDetails);
        }
      } else {
        return false;
      }
    } else if (action === 'Cancel' && moderated) {
      return false;
    } else {
      return true;
    }
  });

  actions = actions.map((action) => {
    if (action === 'Dispute' && orderType === 'sales' && state === 'FULFILLED' && timeSinceOrderTimeout(order)) {
      return 'Claim';
    }
    return action;
  });

  return actions;
};

export const getOrderTitleFromOrderId = orderId => I18n.t('utils.order.order', {id: `${(orderId || '').substring(0, 5)}...`});

export const getOrderBriefFromDetails = (orderId, details, orderType) => {
  if (_.isEmpty(details)) {
    return null;
  }

  const {
    contract: {
      buyerOrder: {
        payment: { moderator, amountCurrency, bigAmount },
        timestamp,
        buyerID,
      },
      vendorListings,
    },
    state,
  } = details;

  const { item, vendorID, slug } = vendorListings[0];
  const thumbnail = _.get(item, 'images[0].small');
  const title = _.get(item, 'title');
  const vendorId = orderType === 'purchases' ? _.get(vendorID, 'peerID') : null;
  const buyerId = orderType === 'sales' ? _.get(buyerID, 'peerID') : null;

  return ({
    moderated: !_.isEmpty(moderator),
    orderId,
    paymentCoin: amountCurrency.code,
    read: true,
    state,
    thumbnail,
    timestamp,
    title,
    total: {
      amount: bigAmount,
    },
    vendorId,
    buyerId,
    slug,
  });
};
