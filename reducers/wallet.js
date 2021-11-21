import { handleActions, createAction } from 'redux-actions';

export const actions = {
  fetchWalletBalance: 'WALLET/FETCH_BALANCE',
  setWalletBalance: 'WALLET/SET_BALANCE',
  fetchTransactions: 'WALLET/FETCH_TRANSACTIONS',
  setGlobalTransactions: 'WALLET/SET_GLOBAL_TRANSACTIONS',
  setCoinTransactions: 'WALLET/SET_COIN_TRANSACTIONS',
  sendFunds: 'WALLET/SEND_FUNDS',
  sendFundsSuccess: 'WALLET/SEND_FUNDS_SUCCESS',
  sendFundsFailure: 'WALLET/SEND_FUNDS_FAILURE',
  resetStatus: 'WALLET/RESET_STATUS',
  fetchCoinAddresses: 'WALLET/FETCH_COIN_ADDRESS',
  setCoinAddresses: 'WALLET/SET_COIN_ADDRESS',
  getFees: 'WALLET/GET_FEES',
  setFees: 'WALLET/SET_FEES',
  setTxid: 'WALLET/SET_TXID',
};

export const fetchWalletBalance = createAction(actions.fetchWalletBalance);
export const fetchTransactions = createAction(actions.fetchTransactions);
export const sendFunds = createAction(actions.sendFunds);
export const getFees = createAction(actions.getFees);
export const resetStatus = createAction(actions.resetStatus);

const initialState = {
  balance: {
    BTC: {
      confirmed: '0',
      currency: {
        code: 'BTC',
        currencyType: 'crypto',
        divisibility: 8,
        name: 'Bitcoin',
      },
      height: 0,
      unconfirmed: '0',
    },
    BCH: {
      confirmed: '0',
      currency: {
        code: 'BCH',
        currencyType: 'crypto',
        divisibility: 8,
        name: 'Bitcoin Cash',
      },
      height: 0,
      unconfirmed: '0',
    },
    CFX: {
      confirmed: '0',
      currency: {
        code: 'CFX',
        currencyType: 'crypto',
        divisibility: 18,
        name: 'Conflux',
      },
      height: 0,
      unconfirmed: '0',
    },
    LTC: {
      confirmed: '0',
      currency: {
        code: 'LTC',
        currencyType: 'crypto',
        divisibility: 8,
        name: 'Litecoin',
      },
      height: 0,
      unconfirmed: '0',
    },
    // ZEC: {
    //   confirmed: '0',
    //   currency: {
    //     code: 'ZEC',
    //     currencyType: 'crypto',
    //     divisibility: 8,
    //     name: 'Zcash',
    //   },
    //   height: 0,
    //   unconfirmed: '0',
    // },
    // ETH: {
    //   confirmed: '0',
    //   currency: {
    //     code: 'ETH',
    //     currencyType: 'crypto',
    //     divisibility: 8,
    //     name: 'Ethereum',
    //   },
    //   height: 0,
    //   unconfirmed: '0',
    // },
  },
  transactions: {
    BTC: {},
    BCH: {},
    LTC: {},
    // ZEC: {},
    // ETH: {},
    CFX: {},
  },
  coinAddress: {
    BTC: '',
    BCH: '',
    LTC: '',
    // ZEC: '',
    // ETH: '',
    CFX: '',
  },
  fees: {},
  status: '',
  reason: '',
  sentTxid: '',
  fetchingBalance: false,
  fetchingTransaction: false,
};

export default handleActions(
  {
    [actions.fetchTransactions]: state => ({ ...state, fetchingTransaction: true }),
    [actions.fetchWalletBalance]: state => ({ ...state, fetchingBalance: true }),
    [actions.sendFunds]: state => ({ ...state, status: 'sending' }),
    [actions.sendFundsSuccess]: state => ({ ...state, status: 'send_succeed' }),
    [actions.sendFundsFailure]: (state, action) => ({ ...state, status: 'send_failed', reason: action.payload }),
    [actions.resetStatus]: state => ({ ...state, status: '', reason: '' }),
    [actions.setWalletBalance]: (state, action) => ({
      ...state,
      balance: action.payload,
      fetchingBalance: false,
    }),
    [actions.setGlobalTransactions]: (state, action) => ({
      ...state,
      transactions: action.payload,
    }),
    [actions.setCoinTransactions]: (state, action) => {
      const { coin, transactions } = action.payload;
      const newGlobalTransactions = { ...state.transactions };
      newGlobalTransactions[coin] = transactions;
      return {
        ...state,
        transactions: newGlobalTransactions,
        fetchingTransaction: false,
      };
    },
    [actions.setCoinAddresses]: (state, action) => ({
      ...state,
      coinAddress: action.payload,
    }),
    [actions.setFees]: (state, { payload }) => ({
      ...state,
      fees: payload,
    }),
    [actions.setTxid]: (state, { payload }) => ({
      ...state,
      sentTxid: payload,
    }),
  },
  initialState,
);
