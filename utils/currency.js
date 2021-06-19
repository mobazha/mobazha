/* eslint-disable radix */
import _ from 'lodash';
import BigNumber from 'bignumber.js';

import localCurrencies from '../config/localCurrencies.json';
import { COINS } from '../utils/coins';

// Send money utils
export const removeZeros = (before) => {
  let after = before;
  if (after.indexOf('.') > -1) {
    after = after.replace(/[0]+$/, ''); // Remove trailing 0's
  }
  after = after.replace(/[.]$/, ''); // Remove trailing decimal
  return after;
};

export const getDecimalPoints = (coin) => {
  if (['BTC', 'BCH', 'LTC', 'ZEC'].includes(coin)) {
    return 8;
  } else if (coin === 'ETH' || coin === 'CFX') {
    return 18;
  } else {
    return 2;
  }
};

export const getFixedCurrency = (value, decimalPoints) => (
  BigNumber(value).toFixed(decimalPoints)
);

export const parseCurrencyStr = amountStr => parseFloat(_.isEmpty(amountStr) ? 0 : amountStr);

// Main utils
export const isCrypto = currency => !!COINS[currency];

const satoshiAmount = BigNumber(100000000);
const weiAmount = BigNumber(1000000000000000000); // dripAmount
const centAmount = BigNumber(100);

export const getCurrencySymbol = (localCurrency) => {
  const idx = _.findIndex(localCurrencies, o => o.code === localCurrency);
  return idx >= 0 ? localCurrencies[idx].symbol : '';
};

export const exchangeCurrency = (rates, localCurrency, currency, amount) => {
  const rate = BigNumber(rates[currency === 'VEF' ? 'VES' : currency]);
  const localRate = BigNumber(rates[localCurrency === 'VEF' ? 'VES' : localCurrency]);
  const bgAmount = BigNumber(amount);
  if (isCrypto(currency)) {
    const minUnitAmount = (currency === 'ETH' || currency === 'CFX') ? weiAmount : satoshiAmount;
    return bgAmount.dividedBy(rate).multipliedBy(localRate).dividedBy(minUnitAmount);
  } else {
    return bgAmount.dividedBy(rate).multipliedBy(localRate).dividedBy(centAmount);
  }
};

export const exchangeToBCH = (rates, localCurrency, currency, amount) => {
  const minUnitAmount = (currency === 'ETH' || currency === 'CFX') ? weiAmount : satoshiAmount;
  const rate = BigNumber(rates[currency === 'VEF' ? 'VES' : currency]);
  const localRate = BigNumber(rates[localCurrency === 'VEF' ? 'VES' : localCurrency]);
  const bgAmount = BigNumber(amount);
  return bgAmount.multipliedBy(rate).dividedBy(localRate).multipliedBy(minUnitAmount);
};

export const calculateBalanceFromBCH = (rates, localCurrency, currency, balance) => {
  try {
    const { confirmed, unconfirmed } = balance;
    const cAmount = BigNumber(confirmed);
    const ucAmount = BigNumber(unconfirmed);
    const cBalance = exchangeCurrency(rates, localCurrency, currency, cAmount.plus(ucAmount));
    const pending = exchangeCurrency(rates, localCurrency, currency, ucAmount);
    return { cBalance, pending };
  } catch (e) {
    return { cBalance: BigNumber(0), pending: BigNumber(0) };
  }
};

export const minUnitAmountToBCH = (amount, coin, truncate = true) => {
  const minUnitAmount = (coin === 'ETH' || coin === 'CFX') ? weiAmount : satoshiAmount;
  return BigNumber(truncate ? BigNumber(amount).integerValue() : amount).dividedBy(minUnitAmount);
};

export const getGeneralCoinInfo = coin => COINS[coin];

export const getPriceInMinimumUnit = (price, currency) => {
  const bgPrice = BigNumber(price);
  if (currency === 'BTC' || currency === 'BCH' || currency === 'LTC' || currency === 'ZEC') {
    return bgPrice.multipliedBy(satoshiAmount).toFixed(0);
  }

  if (currency === 'ETH' || currency === 'CFX') {
    return bgPrice.multipliedBy(weiAmount).toFixed(0);
  }

  return bgPrice.multipliedBy(100).toFixed(0);
};

export const getBigCurrencyInfo = (currency, isReduced) => (
  isReduced ? {
    code: currency,
    divisibility: getDecimalPoints(currency),
  } : {
    code: currency,
    divisibility: getDecimalPoints(currency),
    name: localCurrencies.find(o => o.code === currency).name,
    currencyType: isCrypto(currency) ? 'crypto' : 'fiat',
  }
);
