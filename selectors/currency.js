import { createSelector } from 'reselect';
import { isEmpty } from 'lodash';
import BigNumber from 'bignumber.js';

import {
  exchangeCurrency,
  exchangeToBCH,
  calculateBalanceFromBCH,
  getCurrencySymbol,
  removeZeros,
} from '../utils/currency';

const ratesSelector = state => state.exchangeRate.rates;
const localCurrencySelector = state => state.settings.localCurrency;

const localDecimalPointsIfCryptoSelector = createSelector(
  localCurrencySelector,
  (localCurrency) => {
    if (['BTC', 'BCH', 'LTC', 'ZEC'].includes(localCurrency)) {
      return 8;
    } else if (localCurrency === 'ETH' || localCurrency === 'CFX') {
      return 18;
    } else {
      return 0;
    }
  },
);

const convertorSelector = createSelector(
  ratesSelector,
  localCurrencySelector,
  (rates, localCurrency) => (amount, currency) =>
    exchangeCurrency(rates, localCurrency, currency, amount),
);

const convertBetweenCurrenciesSelector = createSelector(
  ratesSelector,
  rates => (amount, baseCurrency, targetCurrency) =>
    exchangeCurrency(rates, targetCurrency, baseCurrency, amount),
);

const convertorFromBCHSelector = createSelector(
  ratesSelector,
  localCurrencySelector,
  (rates, localCurrency) => (amount, baseCoin = 'BCH') =>
    exchangeCurrency(rates, localCurrency, baseCoin, amount),
);

const convertorToBCHSelector = createSelector(
  ratesSelector,
  localCurrencySelector,
  (rates, localCurrency) => (amount, baseCoin = 'BCH') =>
    exchangeToBCH(rates, localCurrency, baseCoin, amount),
);

const convertorBalanceFromBCHSelector = createSelector(
  ratesSelector,
  localCurrencySelector,
  (rates, localCurrency) => (amount, coin = 'BCH') =>
    calculateBalanceFromBCH(rates, localCurrency, coin, amount),
);

const localSymbolSelector = createSelector(localCurrencySelector, localCurrency =>
  getCurrencySymbol(localCurrency));

const localLabelFromBCHSelector = createSelector(
  ratesSelector,
  localSymbolSelector,
  convertorFromBCHSelector,
  localDecimalPointsIfCryptoSelector,
  (rates, localSymbol, convertCurrencyFromBCH, localDecimalPointsIfCrypto) => (price, coin = 'BTC') => {
    if (isEmpty(rates)) {
      return 'calculating...';
    }

    // if price is smaller than 0, it is between -1 and 0, so we make it 0
    const localPrice = convertCurrencyFromBCH(price, coin);
    // Currency-specific formatting
    if (localDecimalPointsIfCrypto) {
      return `${localSymbol}${removeZeros(localPrice.toFixed(localDecimalPointsIfCrypto))}`;
    } else {
      return `${localSymbol}${localPrice.toFixed(2).replace(/\d(?=(\d{3})+\.)/g, '$&,')}`;
    }
  },
);

const localLabelFromLocalSelector = createSelector(
  localSymbolSelector,
  localDecimalPointsIfCryptoSelector,
  (localSymbol, localDecimalPointsIfCrypto) => (price) => {
    const bgPrice = BigNumber(price);
    if (localDecimalPointsIfCrypto) {
      return `${localSymbol}${removeZeros(bgPrice.toFixed(localDecimalPointsIfCrypto))}`;
    } else {
      return `${localSymbol}${bgPrice.toFixed(2)}`;
    }
  },
);

const localMaskSelector = createSelector(
  localSymbolSelector,
  localDecimalPointsIfCryptoSelector,
  (localSymbol, localDecimalPointsIfCrypto) => {
    if (localDecimalPointsIfCrypto) {
      return `${localSymbol}[9999999]{.}[${'9'.repeat(localDecimalPointsIfCrypto)}]`;
    } else {
      return `${localSymbol}[9999999]{.}[99]`;
    }
  },
);

export const convertorsMap = state => ({
  convertCurrency: convertorSelector(state),
  convertBetweenCurrencies: convertBetweenCurrenciesSelector(state),
  convertCurrencyFromBCH: convertorFromBCHSelector(state),
  convertCurrencyToBCH: convertorToBCHSelector(state),
  convertBalanceFromBCH: convertorBalanceFromBCHSelector(state),
  localCurrency: localCurrencySelector(state),
  localDecimalPointsIfCrypto: localDecimalPointsIfCryptoSelector(state),
  localSymbol: localSymbolSelector(state),
  localMask: localMaskSelector(state),
  localLabelFromBCH: localLabelFromBCHSelector(state),
  localLabelFromLocal: localLabelFromLocalSelector(state),
});
