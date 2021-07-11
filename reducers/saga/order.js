/* eslint-disable no-restricted-syntax */
import { call, put, takeEvery, select } from 'redux-saga/effects';
import { hasIn, get } from 'lodash';
import { Alert } from 'react-native';

import { actions as walletActions } from '../wallet';
import { actions } from '../order';
import { actions as StreamActions } from '../stream';
import { actions as socketActions } from '../socket';

import {
  getPurchases,
  getSales,
  confirmOrder,
  cancelOrder,
  fundOrder,
  fulfillOrder,
  refundOrder,
  completeOrder,
  openDispute,
  getOrderDetails,
  acceptDispute,
  claimPayment,
  scanOfflineMessages,
  resendOrderMessage,
} from '../../api/orders';
import { purchaseItem } from '../../api/checkout';
import { resyncBlockchain } from '../../api/wallet';
import { getUserCredentails, getOrderPollForSessionExecuted, getOrderPolls } from './sagaSelector';
import { actions as profilesActions } from '../profiles';
import { timeSinceInSeconds } from '../../utils/time';
import { EXPIRE_IN_HOURS } from '../../utils/order';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const TESTING_PAYMENT_FINALIZED = false;

const delayedAlert = message => setTimeout(() => Alert.alert(message), 1500);


const PURCHASE_STATE_TO_MESSAGE_MAPPING = {
  PENDING: 'ORDER_PAYMENT',
  CANCELED: 'ORDER_CANCEL',
  AWAITING_FULFILLMENT: 'ORDER_PAYMENT',
  COMPLETED: 'ORDER_COMPLETION',
  DISPUTED: 'DISPUTE_OPEN',
  RESOLVED: 'DISPUTE_CLOSE',
};

const SALE_STATE_TO_MESSAGE_MAPPING = {
  DECLINED: 'ORDER_REJECT',
  FULFILLED: 'ORDER_FULFILLMENT',
  DISPUTED: 'DISPUTE_OPEN',
  RESOLVED: 'DISPUTE_CLOSE',
};

function* fetchPurchasesAction(action) {
  const { shouldFetchProfiles, onComplete } = action.payload || {};
  const { username, password } = yield select(getUserCredentails);
  const result = yield call(getPurchases, username, password);
  const purchases = TESTING_PAYMENT_FINALIZED ? (
    (get(result, 'purchases') || []).map(order => (order.state === 'RESOLVED' ? { ...order, state: 'PAYMENT_FINALIZED' } : order))
  ) : (
    get(result, 'purchases') || []
  );

  const disputedOrders = purchases.filter(purchase => purchase.state === 'DISPUTED');
  for (const purchase of disputedOrders) {
    yield put({
      type: actions.updateOrderWithDetails,
      payload: { orderId: purchase.orderId, orderType: 'purchases' },
    });
  }

  if (shouldFetchProfiles) {
    yield put({
      type: profilesActions.fetchProfiles,
      payload: { peerIDs: purchases.map(purchase => purchase.vendorId) },
    });
  }
  yield put({ type: actions.setPurchases, payload: purchases });

  if (onComplete) {
    onComplete();
  }
}

function* fetchSalesAction(action) {
  const { shouldFetchProfiles, onComplete } = action.payload || {};
  const { username, password } = yield select(getUserCredentails);
  const result = yield call(getSales, username, password);

  const sales = TESTING_PAYMENT_FINALIZED ? (
    (get(result, 'sales') || []).map(order => (order.state === 'RESOLVED' ? { ...order, state: 'PAYMENT_FINALIZED' } : order))
  ) : (
    get(result, 'sales') || []
  );

  const disputedOrders = sales.filter(sale => sale.state === 'DISPUTED');
  for (const sale of disputedOrders) {
    yield put({
      type: actions.updateOrderWithDetails,
      payload: { orderId: sale.orderId, orderType: 'sales' },
    });
  }

  if (shouldFetchProfiles) {
    yield put({
      type: profilesActions.fetchProfiles,
      payload: { peerIDs: sales.map(sale => sale.buyerId) },
    });
  }
  yield put({ type: actions.setSales, payload: sales });

  if (onComplete) {
    onComplete();
  }
}

function* fetchOrderAction(action) {
  const { username, password } = yield select(getUserCredentails);
  try {
    const result = yield call(getOrderDetails, username, password, action.payload);
    const { state, contract } = result;
    if (contract) {
      const disputeTimestamp = get(contract, 'dispute.timestamp');
      if (state === 'DISPUTED' && disputeTimestamp && timeSinceInSeconds(new Date(disputeTimestamp)) > EXPIRE_IN_HOURS * 3600) {
        yield put({
          type: actions.setOrder,
          payload: {
            ...result,
            state: 'DISPUTE_EXPIRED',
          },
        });
      } else if (state === 'RESOLVED' && TESTING_PAYMENT_FINALIZED) {
        yield put({
          type: actions.setOrder,
          payload: {
            ...result,
            state: 'PAYMENT_FINALIZED',
          },
        });
      } else {
        yield put({ type: actions.setOrder, payload: result });
      }
    }
  } catch (err) {
    delayedAlert('Failed to fetch order');
  }
}

function* updateOrderWithDetailsAction(action) {
  const { orderId, orderType } = action.payload;
  const { username, password } = yield select(getUserCredentails);
  try {
    const result = yield call(getOrderDetails, username, password, orderId);
    const { state, contract } = result;
    if (contract) {
      const disputeTimestamp = get(contract, 'dispute.timestamp');
      if (state === 'DISPUTED' && disputeTimestamp && timeSinceInSeconds(new Date(disputeTimestamp)) > EXPIRE_IN_HOURS * 3600) {
        yield put({
          type: orderType === 'purchases' ? actions.updatePurchase : actions.updateSale,
          payload: { orderId, body: { state: 'DISPUTE_EXPIRED' } },
        });
      }
    }
  } catch (err) {
    delayedAlert('Failed to fetch order');
  }
}

function* sendNotification(peerID, orderId, type) {
  let orderContent = '';
  switch (type) {
    case 'confirmed':
      orderContent = 'Your order is being processed.';
      break;
    case 'cancelled':
      orderContent = 'The order was cancelled by the buyer.';
      break;
    case 'fulfilled':
      orderContent = 'Your item has been dispatched.';
      break;
    case 'funded':
      orderContent = "You've made a sale. Congratulations!";
      break;
    case 'openDispute':
      orderContent = 'One of your orders is in dispute and is being reviewed by the moderator.';
      break;
    case 'resolveDispute':
      orderContent = 'The disputed order is resolved, and funds have been released.';
      break;
    case 'releaseEscrow':
      orderContent = 'Funds for an order in escrow were released.';
      break;
    case 'refunded':
      orderContent = 'The order was cancelled and refunded.';
      break;
    case 'completed':
    default:
      orderContent = 'Funds have been transferred to your account.';
      break;
  }
  yield put({ type: StreamActions.sendNotification,
    payload: {
      verb: 'order',
      type,
      peerID,
      content: { orderId, body: orderContent },
    },
  });
}

function* completeOrderAction(action) {
  const { username, password } = yield select(getUserCredentails);
  const { vendorId, ...payload } = action.payload;
  yield put({ type: actions.actionStart });
  try {
    const response = yield call(completeOrder, username, password, payload);
    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      eventTracker.trackEvent('Order-Buyer-CompleteOrder');
      yield put({ type: actions.fetchSales });
      yield put({ type: actions.fetchOrder, payload: payload.orderId });
    }
    yield call(sendNotification, vendorId, payload.orderId, 'completed');
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* confirmOrderAction(action) {
  const { username, password } = yield select(getUserCredentails);
  const { peerID, ...payload } = action.payload;
  yield put({ type: actions.actionStart });
  try {
    const response = yield call(confirmOrder, username, password, payload);
    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      const { orderId, reject } = payload;
      if (reject) {
        eventTracker.trackEvent('Order-Vendor-DeclineOrder');
      } else {
        eventTracker.trackEvent('Order-Vendor-AcceptOrder');
      }
      yield put({ type: actions.fetchSales });
      yield put({ type: actions.fetchOrder, payload: orderId });
      yield call(sendNotification, peerID, orderId, 'confirmed');
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* cancelOrderAction(action) {
  const { username, password } = yield select(getUserCredentails);
  const { peerID, orderId } = action.payload;
  yield put({ type: actions.actionStart });
  try {
    const response = yield call(cancelOrder, username, password, orderId);
    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      eventTracker.trackEvent('Order-Buyer-CancelOrder');
      yield put({ type: actions.fetchPurchases });
      yield put({ type: walletActions.fetchWalletBalance });
      yield put({ type: actions.fetchOrder, payload: orderId });
      yield call(sendNotification, peerID, orderId, 'cancelled');
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* fulfillOrderAction(action) {
  const { body, onSuccess, onFailure, peerID } = action.payload;
  const { username, password } = yield select(getUserCredentails);
  yield put({ type: actions.actionStart });
  try {
    const response = yield call(fulfillOrder, username, password, body);
    if (hasIn(response, 'success') && response.success === false) {
      if (onFailure) {
        onFailure();
      }
      delayedAlert(response.reason);
    } else {
      const { orderId } = body;
      eventTracker.trackEvent('Order-Vendor-FulfillOrder');
      yield put({ type: actions.fetchSales });
      yield put({ type: actions.fetchOrder, payload: orderId });
      yield call(sendNotification, peerID, orderId, 'fulfilled');
      if (onSuccess) {
        onSuccess();
      }
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* fundOrderAction(action) {
  const { username, password } = yield select(getUserCredentails);
  yield put({ type: actions.actionStart });
  const { peerID, ...payload } = action.payload;
  try {
    const response = yield call(fundOrder, username, password, payload);

    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      yield put({ type: actions.fetchPurchases });
      yield put({ type: walletActions.fetchWalletBalance });
      yield put({ type: actions.fetchOrder, payload: payload.orderId });
      yield call(sendNotification, peerID, payload.orderId, 'funded');
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* openDisputeAction(action) {
  const { username, password } = yield select(getUserCredentails);
  const {
    orderId, claim, orderType, callback, vendorId,
  } = action.payload;
  yield put({ type: actions.actionStart });

  try {
    const response = yield call(openDispute, username, password, orderId, claim);

    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      if (callback) {
        callback();
      }
      if (orderType === 'purchases') {
        eventTracker.trackEvent('Order-Buyer-OpenDispute');
      } else {
        eventTracker.trackEvent('Order-Vendor-OpenDispute');
      }
      yield put({ type: actions.fetchPurchases });
      yield put({ type: actions.fetchSales });
      yield put({ type: actions.fetchOrder, payload: orderId });
      yield call(sendNotification, vendorId, orderId, 'openDispute');
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* acceptPayoutAction(action) {
  const { username, password } = yield select(getUserCredentails);
  const { orderId, orderType, buyerId, vendorId } = action.payload;
  yield put({ type: actions.actionStart });

  try {
    const response = yield call(acceptDispute, username, password, orderId);

    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      if (orderType === 'purchases') {
        eventTracker.trackEvent('Order-Buyer-AcceptPayout');
      } else {
        eventTracker.trackEvent('Order-Vendor-AcceptPayout');
      }
      yield put({ type: actions.fetchPurchases });
      yield put({ type: actions.fetchSales });
      yield put({ type: actions.fetchOrder, payload: orderId });
      yield call(sendNotification, orderType === 'purchases' ? vendorId : buyerId, orderId, 'resolveDispute');
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* claimPaymentAction(action) {
  const { username, password } = yield select(getUserCredentails);
  const { orderId, vendorId } = action.payload;
  yield put({ type: actions.actionStart });

  try {
    const response = yield call(claimPayment, username, password, orderId);

    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      delayedAlert('You claimed Payment!');
      yield put({ type: actions.fetchPurchases });
      yield put({ type: actions.fetchSales });
      yield put({ type: actions.fetchOrder, payload: orderId });
      yield call(sendNotification, vendorId, orderId, 'releaseEscrow');
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* refundOrderAction(action) {
  const { username, password } = yield select(getUserCredentails);
  yield put({ type: actions.actionStart });
  const { orderId, vendorId } = action.payload;
  try {
    const response = yield call(refundOrder, username, password, orderId);
    if (hasIn(response, 'success') && response.success === false) {
      delayedAlert(response.reason);
    } else {
      yield put({ type: actions.fetchSales });
      yield put({ type: walletActions.fetchWalletBalance });
      yield put({ type: actions.fetchOrder, payload: orderId });
      yield call(sendNotification, vendorId, orderId, 'refunded');
    }
  } catch (err) {
    delayedAlert('Error happened because of unknown issues');
  } finally {
    yield put({ type: actions.actionFinished });
  }
}

function* purchaseListingAction(action) {
  const { username, password } = yield select(getUserCredentails);
  const {
    orderData, onSuccess, onHalf, onFailure, walletType, peerID,
  } = action.payload;

  try {
    const response = yield call(purchaseItem, username, password, orderData);
    if (!hasIn(response, 'orderId')) {
      eventTracker.trackEvent('Checkout-PaymentFailed', response.reason);
      yield call(onFailure, response.reason);
    } else {
      if (onHalf) {
        yield call(onHalf, response);
      }

      if (walletType === 'external') {
        return;
      }

      const { amount: amountObj = {}, paymentAddress, orderId } = response;
      const { amount } = amountObj;
      const coin = orderData.paymentCoin;
      const finalResponse = yield call(fundOrder, username, password, {
        amount, address: paymentAddress, orderId, coin, memo: '',
      });
      if (hasIn(finalResponse, 'success') && finalResponse.success === false) {
        eventTracker.trackEvent('Checkout-PaymentFailed', finalResponse.reason);
        yield call(onFailure, finalResponse.reason);
      } else {
        eventTracker.trackEvent('Checkout-PaymentSucceed');
        yield call(onSuccess, finalResponse);
        yield put({ type: actions.fetchPurchases });
        yield put({ type: walletActions.fetchWalletBalance });
        yield put({ type: actions.fetchOrder, payload: orderId });

        yield call(sendNotification, peerID, orderId, 'funded');
      }
    }
  } catch (err) {
    eventTracker.trackEvent('Checkout-PaymentFailed', err);
    if (onFailure) {
      yield call(onFailure, I18n.t('reducers.saga.order.Unknown_error'));
    }
  }
}

function* scanOfflineMessagesAction(action) {
  const { onComplete } = action.payload || {};
  const { username, password } = yield select(getUserCredentails);
  yield call(scanOfflineMessages, username, password);
  if (onComplete) {
    onComplete();
  }
}

function* resendOrderMessageAction(action) {
  const { orderId, messageType, onComplete } = action.payload || {};
  const { username, password } = yield select(getUserCredentails);
  yield call(resendOrderMessage, username, password, orderId, messageType);
  if (onComplete) {
    onComplete();
  }
}

function* incomingSocketMessage(action) {
  const pollForSessionExecuted = yield select(getOrderPollForSessionExecuted);
  const { username, password } = yield select(getUserCredentails);
  if (pollForSessionExecuted) {
    return;
  }

  const { payload } = action;
  const { data } = payload || {};
  const { status } = data || {};
  // console.warn('--------status------', payload);
  if (status === 'publish complete') {
    // console.warn('--------publish complete------');
    yield put({ type: actions.setPollForSessionExecuted });

    yield put({ type: actions.scanOfflineMessages });
    yield call(resyncBlockchain);

    const polls = yield select(getOrderPolls);
    const purchasesResponse = yield call(getPurchases, username, password);
    const purchases = get(purchasesResponse, 'purchases') || [];
    const salesResponse = yield call(getSales, username, password);
    const sales = get(salesResponse, 'sales') || [];
    const orders = [...purchases, ...sales];

    for (let i = 0; i < orders.length; i += 1) {
      const order = orders[i];
      const poll = polls[order.orderId] || {};
      const mapObject = i < purchases.length ? PURCHASE_STATE_TO_MESSAGE_MAPPING : SALE_STATE_TO_MESSAGE_MAPPING;
      const messageType = mapObject[order.state];
      if (!messageType) {
        continue;
      }

      // console.warn('--------poll------', poll.count || 0, order.state, messageType, poll.messageType, order.orderId);
      if ((poll.count || 0) < 3 || messageType !== poll.messageType) {
        yield put({
          type: actions.resendOrderMessage,
          payload: { orderId: order.orderId, messageType },
        });
      }
    }
  }
}

const OrderSaga = function* Search() {
  yield takeEvery(actions.fetchPurchases, fetchPurchasesAction);
  yield takeEvery(actions.fetchSales, fetchSalesAction);
  yield takeEvery(actions.updateOrderWithDetails, updateOrderWithDetailsAction);
  yield takeEvery(actions.fetchOrder, fetchOrderAction);
  yield takeEvery(actions.cancelOrder, cancelOrderAction);
  yield takeEvery(actions.completeOrder, completeOrderAction);
  yield takeEvery(actions.confirmOrder, confirmOrderAction);
  yield takeEvery(actions.fulfillOrder, fulfillOrderAction);
  yield takeEvery(actions.fundOrder, fundOrderAction);
  yield takeEvery(actions.openDispute, openDisputeAction);
  yield takeEvery(actions.refundOrder, refundOrderAction);
  yield takeEvery(actions.acceptPayout, acceptPayoutAction);
  yield takeEvery(actions.claimPayment, claimPaymentAction);
  yield takeEvery(actions.purchaseListing, purchaseListingAction);
  yield takeEvery(actions.scanOfflineMessages, scanOfflineMessagesAction);
  yield takeEvery(actions.resendOrderMessage, resendOrderMessageAction);
  yield takeEvery(socketActions.incomingSocketMessage, incomingSocketMessage);
};

export default OrderSaga;
