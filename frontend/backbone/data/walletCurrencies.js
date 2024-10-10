import _ from 'underscore';
import * as bitcoin from 'bitcoinjs-lib';
import app from '../app';

// If a currency does not support fee bumping or you want to disable it, do not provide a
// feeBumpTransactionSize setting.

export const TIP_ADDRESSES = {
  LTC: app.serverConfig.testnet ? 'tltc1qw4djyk8kha88y63phwpc845z0gaeesq0nvuhft':'ltc1qwl0504xgvfzvxgxsg5xmkan6zuy3n6na3randf',
  BTC: 'bc1qnn0ezj6nh7kvm2pqyatx5hpphkuqvwydmwhw60',
  ZEC: 't1ZHA4i9aquxasN5nhSita11uYyccTQeEYY',
  BCH: app.serverConfig.testnet ? 'qrxmtql3l39n44tnu29ccjy0kjkwr4tx8ywhugjrak':'qp0xmudvwvswlcgh80pt98ysxph6r4wfggzeqh68hr',
  ETH: '0x03bC67c2AEBc572397B19199f540C811F2904718',
  CFX: '0x03bC67c2AEBc572397B19199f540C811F2904718',
  MATIC: '0x03bC67c2AEBc572397B19199f540C811F2904718',
  MATICUSDT: '0x03bC67c2AEBc572397B19199f540C811F2904718',
  MATICUSDC: '0x03bC67c2AEBc572397B19199f540C811F2904718',
  CFX: 'cfx:aaph2me2h30nfwp6ha24zz00xpp187th76e3xurre9',
};

export const isValidAddress = (address, coinType) => {
  if (['btc', 'bch', 'ltc', 'zec'].includes(coinType.toLowerCase())) {
    return isValidBTCLikeAddress(address);
  }

  return isValidETHAddress(address);
}

export const isValidBTCLikeAddress = (address) => {
  try {
    bitcoin.address.fromBech32(address);
    return true;
  } catch (error) {
    try {
      bitcoin.address.fromBase58Check(address, networkParams);
      return true;
    } catch (error) {
      return false;
    }
  }
}

export const isValidETHAddress = (address) => {
  const regex1 = new RegExp('^0x[0-9a-fA-F]{40}$');
  const regex2 = new RegExp('^0x[0-9a-fA-F]{64}$'); // for contract payment address
  return regex1.test(address) || regex2.test(address);
};

let _currencies = [
  {
    code: 'BTC',
    testnetCode: 'BTC',
    symbol: '₿',
    // Not allowing fee bump on BTC right now given the fees.
    // feeBumpTransactionSize: 154,
    qrCodeText: (address) => `bitcoin:${address}`,
    icon: 'imgs/cryptoIcons/BTC.png',
    url: 'https://bitcoin.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://chain.so/address/BTCTEST/${address}`
        : `https://www.oklink.com/btc/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://chain.so/tx/BTCTEST/${txid}`
        : `https://www.oklink.com/btc/tx/${txid}`
    ),
    isValidAddress: isValidBTCLikeAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 60 * 10,
    externallyFundableOrders: true,
  },
  {
    code: 'BCH',
    testnetCode: 'BCH',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      let prefixedAddress = address;

      const prefix = app.serverConfig.testnet ? 'bchtest' : 'bitcoincash';
      prefixedAddress = address.startsWith(prefix)
        ? prefixedAddress : `${prefix}:${address}`;

      return prefixedAddress;
    },
    icon: 'imgs/cryptoIcons/BCH.png',
    url: 'https://bitcoincash.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://explorer.bitcoin.com/tbch/address/bchtest:${address}`
        : `https://www.oklink.com/bch/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://explorer.bitcoin.com/tbch/tx/${txid}`
        : `https://www.oklink.com/bch/tx/${txid}`
    ),
    isValidAddress: isValidBTCLikeAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 60 * 10,
    externallyFundableOrders: true,
  },
  {
    code: 'BNB',
    testnetCode: 'BNB',
    chainName: 'Binance Smart Chain',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/BNB-icon.png',
    url: 'https://bitcoincash.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/address/${address}`
        : `https://bscscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/tx/${txid}`
        : `https://bscscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'BNBUSDT',
    testnetCode: 'BNBUSDT',
    mainChain: 'BNB',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/USDT-icon.png',
    url: 'https://bitcoincash.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/address/${address}`
        : `https://bscscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/tx/${txid}`
        : `https://bscscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'BNBUSDC',
    testnetCode: 'BNBUSDC',
    mainChain: 'BNB',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/USDC-icon.png',
    url: 'https://bitcoincash.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/address/${address}`
        : `https://bscscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/tx/${txid}`
        : `https://bscscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'BNBMBZ',
    testnetCode: 'BNBMBZ',
    mainChain: 'BNB',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/MBZ-icon.png',
    url: 'https://bitcoincash.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/address/${address}`
        : `https://bscscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://testnet.bscscan.com/tx/${txid}`
        : `https://bscscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'MATIC',
    testnetCode: 'MATIC',
    chainName: 'Polygon Chain',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/MATIC-icon.png',
    url: 'https://polygon.technology/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/address/${address}`
        : `https://polygonscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/tx/${txid}`
        : `https://polygonscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'MATICUSDT',
    testnetCode: 'MATICUSDT',
    mainChain: 'MATIC',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/USDT-icon.png',
    url: 'https://polygon.technology/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/address/${address}`
        : `https://polygonscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/tx/${txid}`
        : `https://polygonscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'MATICUSDC',
    testnetCode: 'MATICUSDC',
    mainChain: 'MATIC',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/USDC-icon.png',
    url: 'https://polygon.technology/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/address/${address}`
        : `https://polygonscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/tx/${txid}`
        : `https://polygonscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'MATICMBZ',
    testnetCode: 'MATICMBZ',
    mainChain: 'MATIC',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/MBZ-icon.png',
    url: 'https://polygon.technology/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/address/${address}`
        : `https://polygonscan.com/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://mumbai.polygonscan.com/tx/${txid}`
        : `https://polygonscan.com/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 3,
    externallyFundableOrders: true,
  },
  {
    code: 'CFX',
    testnetCode: 'CFX',
    chainName: 'Conflux eSpace',
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/CFX-icon.png',
    url: 'https://confluxnetwork.org',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/address/${address}`
        : `https://evm.confluxscan.net/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/tx/${txid}`
        : `https://evm.confluxscan.net/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 0.5,
    externallyFundableOrders: false,
  },
  {
    code: 'CFXUSDT',
    testnetCode: 'CFXUSDT',
    mainChain: 'CFX',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/USDT-icon.png',
    url: 'https://confluxnetwork.org',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/address/${address}`
        : `https://evm.confluxscan.net/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/tx/${txid}`
        : `https://evm.confluxscan.net/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 0.5,
    externallyFundableOrders: true,
  },
  {
    code: 'CFXUSDC',
    testnetCode: 'CFXUSDC',
    mainChain: 'CFX',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/USDC-icon.png',
    url: 'https://confluxnetwork.org',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/address/${address}`
        : `https://evm.confluxscan.net/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/tx/${txid}`
        : `https://evm.confluxscan.net/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 0.5,
    externallyFundableOrders: true,
  },
  {
    code: 'CFXMBZ',
    testnetCode: 'CFXMBZ',
    mainChain: 'CFX',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => {
      return address;
    },
    icon: 'imgs/cryptoIcons/MBZ-icon.png',
    url: 'https://confluxnetwork.org',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/address/${address}`
        : `https://evm.confluxscan.net/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://evmtestnet.confluxscan.net/tx/${txid}`
        : `https://evm.confluxscan.net/tx/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 0.5,
    externallyFundableOrders: true,
  },
  {
    code: 'ETH',
    testnetCode: 'ETH',
    qrCodeText: (address) => `ethereum:${address}`,
    icon: 'imgs/cryptoIcons/ETH.png',
    url: 'https://ethereum.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://rinkeby.etherscan.io/address/${address}`
        : `https://blockchair.com/ethereum/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://rinkeby.etherscan.io/tx/${txid}`
        : `https://blockchair.com/ethereum/transaction/${txid}`
    ),
    isValidAddress: isValidETHAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 10,
    externallyFundableOrders: false,
  },
  {
    code: 'LTC',
    testnetCode: 'LTC',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => `litecoin:${address}`,
    icon: 'imgs/cryptoIcons/LTC.png',
    url: 'https://litecoin.org/',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://chain.so/address/LTCTEST/${address}`
        : `https://www.oklink.com/ltc/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://chain.so/tx/LTCTEST/${txid}`
        : `https://www.oklink.com/ltc/tx/${txid}`
    ),
    isValidAddress: isValidBTCLikeAddress,
    supportsEscrowTimeout: true,
    blockTime: 1000 * 60 * 2.5,
    externallyFundableOrders: true,
  },
  {
    code: 'ZEC',
    testnetCode: 'ZEC',
    feeBumpTransactionSize: 154,
    qrCodeText: (address) => `zcash:${address}`,
    icon: 'imgs/cryptoIcons/ZEC.png',
    url: 'https://z.cash',
    getBlockChainAddressUrl: (address, isTestnet) => (
      isTestnet
        ? `https://explorer.testnet.z.cash/address/${address}`
        : `https://www.oklink.com/zec/address/${address}`
    ),
    getBlockChainTxUrl: (txid, isTestnet) => (
      isTestnet
        ? `https://explorer.testnet.z.cash/tx/${txid}`
        : `https://www.oklink.com/zec/tx/${txid}`
    ),
    isValidAddress: isValidBTCLikeAddress,
    supportsEscrowTimeout: false,
    blockTime: 1000 * 60 * 2.5,
    externallyFundableOrders: true,
  },
];

let _initialized = false;

function enforceInitialized() {
  if (!_initialized) {
    throw new Error('This module must be initialized before proceeeding.');
  }
}

let _indexedCurrencies;

function getIndexedCurrencies() {
  if (_indexedCurrencies) return _indexedCurrencies;

  _indexedCurrencies = _currencies
    .reduce((indexedObj, currency) => {
      indexedObj[currency.testnetCode] = { ...currency };
      indexedObj[currency.code] = { ...currency };
      return indexedObj;
    }, {});

  return _indexedCurrencies;
}

export function init(walletCurs, walletCurDef) {
  if (!Array.isArray(walletCurs)) {
    // the wallet curs as provided in the 'wallets' property of 'ob/config'
    throw new Error('Please provide a list of wallet currencies.');
  }

  if (typeof walletCurDef !== 'object') {
    // the wallet cur definition as provided in 'ob/wallet/currencies'
    throw new Error('Please provide the wallet currencies definition as an object.');
  }

  // The final currencies list stored in this module will be a union of
  // the walletCurs, the walletCur def and the initial currencies declared
  // here in the _currencies variable. The currency must be declared in all
  // three for it to remain.
  const curs = [];

  const indexedCurs = getIndexedCurrencies();
  // We don't want the indexed curs cached since the definition is about to change
  _indexedCurrencies = null;

  Object
    .keys(indexedCurs)
    .forEach((curCode) => {
      const curDef = walletCurDef[curCode];

      if (
        curDef
        && walletCurs.includes(curDef.code)
      ) {
        const clientCur = indexedCurs[curDef.code];
        const curData = {
          ...clientCur,
          coinDivisibility: curDef.divisibility,
        };

        curs.push(curData);
      }
    });

  _currencies = curs;
  _initialized = true;
}

function getTranslatedCurrencies(
  lang = (app && app.localSettings && app.localSettings.standardizedTranslatedLang()) || 'en-US',
  sort = true,
) {
  enforceInitialized();

  if (!lang) {
    throw new Error('Please provide the language the translated currencies'
      + ' should be returned in.');
  }

  let translated = _currencies.map((currency) => ({
    ...currency,
    name: app.polyglot.t(`cryptoCurrencies.${currency.code}`),
  }));

  if (sort) {
    translated = translated.sort((a, b) => a.name.localeCompare(b.name, lang));
  }

  return translated;
}

const memoizedGetTranslatedCurrencies = _.memoize(getTranslatedCurrencies, (lang, sort) => `${lang}-${!!sort}`);

export { memoizedGetTranslatedCurrencies as getTranslatedCurrencies };

export function getCurrencyByCode(code) {
  enforceInitialized();

  if (typeof code !== 'string') {
    throw new Error('Please provide a currency code as a string.');
  }

  return getIndexedCurrencies()[code];
}

let currenciesSortedByCode;

export function getCurrenciesSortedByCode() {
  enforceInitialized();

  if (currenciesSortedByCode) {
    return currenciesSortedByCode;
  }

  currenciesSortedByCode = _currencies.sort((a, b) => {
    if (a.code < b.code) return -1;
    if (a.code > b.code) return 1;
    return 0;
  });

  return currenciesSortedByCode;
}

/**
 * Since many of our crypto related mapping (e.g. icons) are done based off of
 * a mainnet code, this function will attempt to obtain the mainnet code if a testnet
 * one is passed in. This only works for crypto coins that we have registered as
 * accepted currencies (i.e. are enumerated in data/cryptoCurrencies), but those are
 * the only ones that should ever come as testnet codes.
 */
export function ensureMainnetCode(cur) {
  enforceInitialized();

  if (typeof cur !== 'string' || !cur.length) {
    throw new Error('Please provide a non-empty string.');
  }

  const curObj = getCurrencyByCode(cur);
  return curObj ? curObj.code : cur;
}

export function getWalletCurs() {
  return _currencies;
}

/**
 * Returns a list of the wallet currency codes supported by the wallet.
 *
 * @param {object} [options={}] - Function options
 * @param {boolean} [options.testnet=apps.serverConfig.testnet] - Indicates if the app
 *   is running on testnet. If so, testnet codes will be returned.
 * @return {Array} An Array containing the currency codes that are supported by the wallet.
 */
export function supportedWalletCurs(options = {}) {
  const opts = {
    testnet: (app && app.serverConfig && app.serverConfig.testnet) || false,
    ...options,
  };

  enforceInitialized();

  return getWalletCurs()
    .filter((cur) => (opts.testnet ? cur.testnetCode : true))
    .map((cur) => (opts.testnet ? cur.testnetCode : cur.code));
}

/**
 * Returns a boolean indicating whether the given code is supported by the wallet.
 *
 * @param {string} cur - A currency code.
 * @param {object} [options={}] - Function options - these are sent to supportedWalletCurs.
 * @return {boolean} A boolean indicating whether the given code is supported by the wallet.
 */
export function isSupportedWalletCur(cur, options = {}) {
  enforceInitialized();

  if (typeof cur !== 'string') {
    throw new Error('Please provide a cur as a string.');
  }

  return supportedWalletCurs(options).includes(cur);
}

/**
 * Given a list of currencies, a filtered list will be returned containing only the
 * currencies in the list that are supported by the wallet
 *
 * @param {Array} curs - A list of currencies to filter.
 * @param {object} [options={}] - Function options - these are sent to isSupportedWalletCur.
 * @return {Array} A list based off the intersection of the giveen curs and the supported
 *   wallt curs.
 */
export function onlySupportedWalletCurs(curs = [], options = {}) {
  enforceInitialized();

  if (!Array.isArray(curs)) {
    throw new Error('Curs must be provided as an Array.');
  }

  if (curs.filter((cur) => (typeof cur !== 'string')).length) {
    throw new Error('Curs items must be provided as strings.');
  }

  return curs.filter((cur) => isSupportedWalletCur(cur, options));
}

/**
 * A proxy for onlySupportedWalletCurs with the difference being that this will
 * return a boolean indicating if any of the provided curs are supported as wallet
 * currencies. (same arguments as onlySupportedWalletCurs).
 * @return {boolean} A boolean indicating if any of the provided curs are supported
 *   as wallet currencies.
 */
export function anySupportedByWallet(...args) {
  enforceInitialized();
  return !!(onlySupportedWalletCurs(...args).length);
}
