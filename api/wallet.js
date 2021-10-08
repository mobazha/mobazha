import { gatewayAPI } from './const';
import { makeRequest } from './common';
import { serverConfig } from '../utils/server';
import './mock';

export const getWalletFailureMessage = (reason) => {
  if (reason === 'ERROR_DUST_AMOUNT') {
    return 'Please enter a valid amount';
  } else if (reason === 'ERROR_INVALID_ADDRESS') {
    return 'The address you\'re sending to appears to be invalid. Please try again.';
  } else if (reason === 'ERROR_INSUFFICIENT_FUNDS') {
    return 'You don\'t have sufficient funds!';
  } else {
    return reason;
  }
};

// Fetch the wallet seed
export const getWalletMnemonic = () => {
  const url = `${gatewayAPI}/wallet/mnemonic`;
  return makeRequest(url);
};

// Fetch the latest Bitcoin address
export const getCoinAddresses = () => {
  const url = `${gatewayAPI}/wallet/address`;
  return makeRequest(url);
};

// Fetch the converted address (for Conflux)
export const getCoinUpdateAddress = (coin, addr) => {
  const url = `${gatewayAPI}/wallet/update_address/${coin}?address=${addr}`;
  return makeRequest(url);
};

// Fetch the wallet balance
export const getWalletBalance = () => {
  const url = `${gatewayAPI}/wallet/balance`;
  return makeRequest(url);
};

// Fetch the fees
export const getFees = () => {
  const url = `${gatewayAPI}/wallet/fees`;
  return makeRequest(url);
};

// Fetch the wallet history
export const getWalletHistory = (coin) => {
  const url = `${gatewayAPI}/wallet/transactions/${coin}`;
  return makeRequest(url);
};

// Fetch the wallet status
export const getWalletStatus = () => {
  const url = `${gatewayAPI}/wallet/status`;
  return makeRequest(url);
};

// Fetch the exchange rate
export const getExchangeRate = (coin = 'btc') => {
  const url = `${gatewayAPI}/ob/exchangerate/${coin.toLowerCase()}`;
  return makeRequest(url);
};

// Send funds
export const sendBitcoins = (body) => {
  const url = `${gatewayAPI}/wallet/spend`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
    body: JSON.stringify(body),
  };
  return fetch(url, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log('Error while send Bitcoins', err);
      return {
        success: false,
      };
    });
};

// Resync the blockchain
export const resyncBlockchain = () => {
  const url = `${gatewayAPI}/wallet/resyncblockchain`;
  const headers = {
    method: 'POST',
    headers: serverConfig.getAuthHeader(),
  };
  return fetch(url, headers).then(response => response.json());
};

// Estimate the transaction fee
export const estimateFee = (coinType, amount, feeLevel) => {
  // Hack - convert superEconomic to super_economic
  const refinedFeeLevel = feeLevel === 'superEconomic' ? 'super_economic' : feeLevel;
  const url = `${gatewayAPI}/wallet/estimatefee/${coinType}?amount=${amount}&feeLevel=${refinedFeeLevel}`;
  const headers = {
    method: 'GET',
    headers: serverConfig.getAuthHeader(),
  };
  return fetch(url, headers)
    .then(response => response.json())
    .catch((err) => {
      console.log('Error while resynch Bitcoins', err);
      return {};
    });
};
