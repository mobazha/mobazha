import { all, call, put, takeEvery } from 'redux-saga/effects';
import { hasIn } from 'lodash';

import { actions } from '../wallet';

import {
  getWalletBalance,
  getWalletHistory,
  sendBitcoins,
  getCoinAddresses,
  getFees,
  getWalletFailureMessage,
} from '../../api/wallet';
import { eventTracker } from '../../utils/EventTracker';

function* fetchBalance() {
  const balance = yield call(getWalletBalance);
  yield put({ type: actions.setWalletBalance, payload: balance });
}

function* fetchTransactions(action) {
  if (!action.payload) {
    const [BTC, BCH, LTC, CFX] = yield all([
      call(getWalletHistory, 'btc'),
      call(getWalletHistory, 'bch'),
      call(getWalletHistory, 'ltc'),
      // call(getWalletHistory, 'zec'),
      // call(getWalletHistory, 'eth'),
      call(getWalletHistory, 'cfx'),
    ]);
    yield put({
      type: actions.setGlobalTransactions,
      payload: {
        BTC, BCH, LTC, CFX,
      },
    });
  } else {
    const coinName = action.payload;
    const transactions = yield call(getWalletHistory, coinName.toLowerCase());
    yield put({
      type: actions.setCoinTransactions,
      payload: { coin: coinName, transactions },
    });
  }
}

function* sendFunds(action) {
  const { data, nextAction } = action.payload;
  const result = yield call(sendBitcoins, data);
  if (hasIn(result, 'confirmedBalance')) {
    if (nextAction) {
      yield call(nextAction);
    }
    eventTracker.trackEvent('Wallet-PaymentSuccess');
    yield put({ type: actions.sendFundsSuccess });
    yield put({ type: actions.fetchWalletBalance });
    yield put({ type: actions.setTxid, payload: result.txid });
    yield put({ type: actions.fetchTransactions, payload: data.wallet });
  } else if (result.success === false) {
    const { reason } = result;
    eventTracker.trackEvent('Wallet-PaymentFailed', reason);
    yield put({ type: actions.sendFundsFailure, payload: getWalletFailureMessage(reason) });
  }
}

function* fetchCoinAddresses() {
  const result = yield call(getCoinAddresses);
  yield put({ type: actions.setCoinAddresses, payload: result });
}

function* fetchTransactionFee() {
  const result = yield call(getFees);
  yield put({
    type: actions.setFees,
    payload: result,
  });
}

const wallet = function* Wallet() {
  yield takeEvery(actions.fetchWalletBalance, fetchBalance);
  yield takeEvery(actions.fetchTransactions, fetchTransactions);
  yield takeEvery(actions.sendFunds, sendFunds);
  yield takeEvery(actions.fetchCoinAddresses, fetchCoinAddresses);
  yield takeEvery(actions.getFees, fetchTransactionFee);
};

export default wallet;
