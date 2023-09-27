<template>
  <div class="modal wallet modalScrollPage">
    <BaseModal @close="onClose">
      <template v-slot:component>
        <div class="flexColRows gutterV">
          <div class="topControls flexRow">
            <div class="contentBox padSm clrP clrBr clrSh3 gutterHTn flexNoShrink">
              <div class="flexVCent walletIconWrap">
                <WalletIcon />
              </div>
              <span>{{ ob.polyT('wallet.title') }}</span>
              <a class="jsModalClose tx6 txU">{{ ob.polyT('wallet.closeLink') }}</a>
            </div>
            <div class="js-tickerContainer tickerContainer flexHRight">
              <CryptoTicker v-if="activeCoin" :coinType="activeCoin" />
            </div>
          </div>
          <div class="flex gutterH">
            <div class="col3">
              <div class="flexColWide gutterV">
                <div class="js-coinNavContainer">
                  <ul class="coinNav unstyled border padMdKids borderStacked clrP clrBr clrSh3">
                    <CoinNavItem
                      v-for="(coin, key) in navCoins"
                      :key="key"
                      :options="{ initialState: { ...coin, active: coin.code === activeCoin } }"
                      @click="coinSelected(coin)"
                    />
                  </ul>
                </div>
                <div class="js-cryptoListingsTeaser border clrP clrBr clrSh3">
                  <CryptoListingsTeaser
                    :viewCryptoListingsUrl="viewCryptoListingsUrl"
                    @createListing="onClickCreateListing"
                    @viewCryptoListings="onClickViewCryptoListings"
                  />
                </div>
              </div>
            </div>
            <div class="col9">
              <div class="flexColWide gutterV">
                <div v-if="activeCoin">
                  <div class="js-coinStatsContainer"></div>
                  <CoinStats :options="{ initialState: coinStatsState }" />
                  <div>
                    <div class="flexColWide clrP clrSh3">
                      <div class="js-sendReceiveNavContainer rowMd"></div>
                      <SendReceiveNav class="rowMd" :sendModeOn="sendModeOn" @clickSend="onClickSend" @clickReceive="onClickReceive" />
                      <div class="js-sendReceiveContainer sendReceiveContainer clrP">
                        <SendMoney v-if="sendModeOn" :options="{ coinType: activeCoin }" />
                        <ReceiveMoney v-else ref="receiveMoney" :coinType="activeCoin" :fetching="fetchingAddress" :address="receiveAddress" />
                      </div>
                    </div>
                  </div>
                  <div class="clrP clrSh3 posR">
                    <div class="js-transactionsContainer">
                      <TransactionsVw
                        ref="transactionsVw"
                        v-if="showTransactionsVw"
                        :options="transactionViewOptions"
                        @bumpFeeAttempt="onBumpFeeAttempt"
                        @bumpFeeSuccess="onBumpFeeSuccess"
                        @postInit="onTransactionsVwPostInit"
                      />
                    </div>
                    <div class="js-reloadTransactionsContainer reloadTransactions">
                      <ReloadTransactions :options="{ initialState: { coinType: activeCoin } }" />
                    </div>
                  </div>
                </div>

                <div v-else>
                  <div class="clrP clrSh3 clrBr border zeroSupportedCurs">
                    <div class="center">{{ ob.polyT('wallet.zeroSupportedCurs') }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import $ from 'jquery';
import bigNumber from 'bignumber.js';
import { isSupportedWalletCur, ensureMainnetCode, supportedWalletCurs } from '../../../../backbone/data/walletCurrencies';
import defaultSearchProviders from '../../../../backbone/data/defaultSearchProviders';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import app from '../../../../backbone/app';
import { launchEditListingModal } from '../../../../backbone/utils/modalManager';
import Transactions from '../../../../backbone/collections/wallet/Transactions';
import Listing from '../../../../backbone/models/listing/Listing';
import CoinNavItem from './CoinNavItem.vue';
import CoinStats from './CoinStats.vue';
import SendReceiveNav from './SendReceiveNav.vue';
import SendMoney from './SendMoney.vue';
import ReceiveMoney from './ReceiveMoney.vue';
import TransactionsVw from './transactions/Transactions.vue';
import ReloadTransactions from './ReloadTransactions.vue';
import CryptoListingsTeaser from './CryptoListingsTeaser.vue';

export default {
  components: {
    CoinNavItem,
    CoinStats,
    SendReceiveNav,
    SendMoney,
    ReceiveMoney,
    ReloadTransactions,
    TransactionsVw,
    CryptoListingsTeaser,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      activeCoin: '',
      viewCryptoListingsUrl: '',
      sendModeOn: true,

      fetchingAddress: true,
      receiveAddress: '',
      transactionsState: {},

      showTransactionsVw: true,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {},
  watch: {
    activeCoin(coin, oldVal) {
      if (this.needAddress[coin]) {
        this.fetchAddress(coin);
      }

      if (this.sendModeOn && !(app.walletBalances.get(coin) && app.walletBalances.get(coin).get('confirmed'))) {
        this.sendModeOn = false;
      }

      this.reRenderTransactionsVw();
    },
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        activeCoin: this.activeCoin,
      };
    },
    displayCur() {
      return (app && app.settings && app.settings.get('localCurrency')) || 'USD';
    },
    coinStatsState() {
      const { activeCoin } = this;
      const balance = app && app.walletBalances && app.walletBalances.get(activeCoin);
      return {
        cryptoCur: ensureMainnetCode(activeCoin),
        confirmed: balance && balance.get('confirmed'),
        unconfirmed: balance && balance.get('unconfirmed'),
        transactionCount: this.transactionsCountActive,
      };
    },
    transactionsCountActive() {
      let coinType = this.activeCoin;
      const transactionsState = this.transactionsState[coinType] || {};
      const cl = transactionsState && transactionsState.cl;
      const newTxs = this.$refs.transactionsVw ? this.$refs.transactionsVw.newTransactionsTXs : {};
      return (cl ? cl.length : 0) + (newTxs ? newTxs?.size ?? 0 : 0);
    },
    transactionViewOptions() {
      let coin = this.activeCoin;
      const transactionsState = this.transactionsState[coin] || { needsFetch: true };
      let cl = transactionsState && transactionsState.cl;

      console.log('coin: ', coin)
      console.log('cl: ', cl)

      if (!cl) {
        cl = new Transactions([], { coinType: coin });
        transactionsState.cl = cl;

        this.listenToOnce(cl, 'sync', (md, response, options) => {
          if (options && options.xhr) {
            options.xhr.done((data) => {
              transactionsState.needsFetch = false;
              this.setCountAtFirstFetch(data.count, coin);
            });
          }
        });

        this.listenToOnce(cl, 'reset', () => {
          this.listenToOnce(cl, 'sync', (md, response, options) => {
            if (options && options.xhr) {
              options.xhr.done((data) => {
                this.setCountAtFirstFetch(data.count, coin);
              });
            }
          });
        });
      }

      return {
        collection: transactionsState.cl,
        // $scrollContainer: this.$el,
        fetchOnInit: transactionsState.needsFetch,
        countAtFirstFetch: transactionsState.countAtFirstFetch,
        bumpFeeXhrs: transactionsState.bumpFeeAttempts || undefined,
      };
    },
  },
  methods: {
    loadData(options = {}) {
      const navCoins = supportedWalletCurs({ clientSupported: false }).sort((a, b) => {
        const aSortVal = app.polyglot.t(`cryptoCurrencies.${a}`, { _: a });
        const bSortVal = app.polyglot.t(`cryptoCurrencies.${b}`, { _: b });

        return aSortVal.localeCompare(bSortVal, app.localSettings.standardizedTranslatedLang(), { sensitivity: 'base' });
      });

      let initialActiveCoin;

      if (options.initialActiveCoin && typeof options.initialActiveCoin === 'string') {
        initialActiveCoin = isSupportedWalletCur(options.initialActiveCoin) ? options.initialActiveCoin : null;
      }

      if (!initialActiveCoin) {
        initialActiveCoin = navCoins.find((coin) => isSupportedWalletCur(coin)) || null;
      }

      // If at this point the initialActiveCoin and consequently this.activeCoin
      // are null, it indicates that none of the wallet currencies are supported by
      // this client.

      const opts = {
        initialSendModeOn: (app.walletBalances.get(initialActiveCoin) && app.walletBalances.get(initialActiveCoin).get('confirmed')) || false,
        ...options,
        initialActiveCoin,
      };

      this.setState(opts.initialState || {});
      this.activeCoin = opts.initialActiveCoin;

      this.addressFetches = {};
      this.needAddress = navCoins.reduce((acc, coin) => {
        acc[coin] = true;
        return acc;
      }, {});
      // The majority of the TransactionsVw state is managed within the component, but
      // some of it we'll manage so as you nav from coin to coin, certain state is maintained.
      this.transactionsState = navCoins.reduce((acc, coin) => {
        acc[coin] = { needsFetch: true };
        return acc;
      }, {});
      this.popInTimeouts = [];

      this.navCoins = navCoins.map((coin) => {
        const balanceMd = app.walletBalances.get(coin);
        return {
          active: coin === opts.initialNavCoin,
          code: coin,
          name: app.polyglot.t(`cryptoCurrencies.${coin}`, { _: coin }),
          balance: balanceMd && balanceMd.get('confirmed'),
          clientSupported: isSupportedWalletCur(coin),
        };
      });

      const ob1ProviderData = defaultSearchProviders.find((provider) => provider.id === 'mbz');
      this.viewCryptoListingsUrl = ob1ProviderData ? `#search?providerQ=${ob1ProviderData.listings}?type=cryptocurrency` : null;

      const serverSocket = getSocket();

      if (initialActiveCoin && serverSocket) {
        this.listenTo(serverSocket, 'message', (e) => {
          if (e.jsonData.wallet && e.jsonData.wallet.transaction) {
            let walletCur;

            try {
              walletCur = e.jsonData.wallet.transaction.CurrencyCode;
            } catch (err) {
              // pass
              console.error('Unable to process a "wallet" socket because the wallet currency ' + 'could not be determined');
              return;
            }

            const cl = (this.transactionsState[walletCur] && this.transactionsState[walletCur].cl) || null;
            if (cl) {
              const data = e.jsonData.wallet.transaction;
              const transaction = cl.get(data.transactionID);

              if (transaction) {
                // existing transaction has been confirmed
                transaction.set(transaction.parse(data));
              } else if (this.activeCoin !== walletCur) {
                this.transactionsState[walletCur].needsFetch = true;
              } else if (this.$refs.transactionsVw) {
                // This is a bit ugly... but most incoming transactions (ones sent via our UI)
                // are immediately added to the collection when their respective APIs succeeds and
                // therefore should not be included in the "new transactions" pop in count.
                // But, at this point we don't know if this is such a transaction, so we'll
                // check back in a bit and see if it's already been added or not. It's a matter
                // of the socket coming in before the AJAX call returns.
                const timeout = setTimeout(() => {
                  if (this.activeCoin === walletCur) {
                    if (!cl.get(e.jsonData.wallet.transaction.transactionID)) {
                      // A new transaction for the active coin - rather than just add it to the
                      // collection causing a page jump, we'll utilize the new transaction pop-up.
                      this.$refs.transactionsVw.newTransactionsTXs.add(e.jsonData.wallet.transaction.transactionID);
                      this.$refs.transactionsVw.showNewTransactionPopup();
                    }
                  } else {
                    this.transactionsState[walletCur].needsFetch = true;
                  }
                }, 1500);

                this.popInTimeouts.push(timeout);
              }
              this.updateTransactionsCount(walletCur);
            }

            if (!e.jsonData.wallet.transaction.height) {
              // new transactions

              if (bigNumber(e.jsonData.wallet.transaction.value).gt(0)) {
                // for incoming new transactions, we'll need a new receiving address
                if (this.activeCoin === walletCur) {
                  this.fetchAddress();
                } else {
                  this.needAddress[walletCur] = true;
                }
              }
            }
          }
        });
      }

      app.walletBalances.forEach((balanceMd) => {
        this.listenTo(balanceMd, 'change:confirmed change:unconfirmed', _.debounce(this.onBalanceChange, 1));
      });

      if (initialActiveCoin) this.fetchAddress();
    },

    onClose() {
      this.$emit('close');
    },

    coinSelected(coin) {
      if (!coin.active && coin.clientSupported) {
        this.activeCoin = coin.code;
      }
    },

    onBalanceChange(md) {
      this.navCoins = this.navCoins.map((navCoin) => ({
        ...navCoin,
        balance: md.id === navCoin.code ? md.get('confirmed') : navCoin.balance,
      }));

      this.coinNav.setState({ coins: this.navCoins });
    },

    onClickCreateListing() {
      const model = new Listing({
        metadata: {
          contractType: 'CRYPTOCURRENCY',
        },
      });

      recordEvent('Listing_NewCryptoFromWallet');

      launchEditListingModal({ model });
    },

    onClickViewCryptoListings() {
      recordEvent('Wallet_ViewCryptoListings');
    },

    onClickSend() {
      this.sendModeOn = true;
      console.log('sendModeOn', this.sendModeOn);
    },

    onClickReceive() {
      this.sendModeOn = false;
      console.log('sendModeOn', this.sendModeOn);
    },

    checkCoinType(coinType) {
      if (typeof coinType !== 'string' || !coinType) {
        throw new Error('Please provide the coinType as a string.');
      }
    },

    fetchAddress(coinType = this.activeCoin) {
      this.checkCoinType(coinType);

      if (this.addressFetches[coinType]) {
        const pendingFetch = this.addressFetches[coinType].find((xhr) => xhr.state() === 'pending');
        if (pendingFetch) return pendingFetch;
      }
      this.fetchingAddress = true;

      this.needAddress[coinType] = false;

      const fetch = $.get(app.getServerUrl(`wallet/address/${coinType}`))
        .done((data) => {
          this.fetchingAddress = false;
          this.receiveAddress = data.address;
        })
        .fail((xhr) => {
          if (xhr.statusText === 'abort') return;
          this.needAddress[coinType] = true;

          this.fetchingAddress = false;
        });

      this.addressFetches[coinType] = this.addressFetches[coinType] || [];
      this.addressFetches[coinType].push(fetch);

      return fetch;
    },

    open(...args) {
      const returnVal = super.open(...args);
      if (this.sendModeOn) {
        const sendVw = this.getSendMoneyVw();
        if (sendVw) sendVw.focusAddress();
      }
      return returnVal;
    },

    remove() {
      Object.keys(this.addressFetches).forEach((coinType) => {
        this.addressFetches[coinType].forEach((fetch) => fetch.abort());
      });
      Object.keys(this.transactionsState).forEach((coinType) => {
        if (this.transactionsState[coinType] && typeof this.transactionsState[coinType].bumpFeeAttempts === 'object') {
          Object.keys(this.transactionsState[coinType].bumpFeeAttempts).forEach((txId) => this.transactionsState[coinType].bumpFeeAttempts[txId].abort());
        }
      });
      this.popInTimeouts.forEach((timeout) => timeout.remove());
      super.remove();
    },

    onBumpFeeAttempt(e) {
      const transactionsState = this.transactionsState[this.activeCoin];

      transactionsState.bumpFeeAttempts = transactionsState.bumpFeeAttempts || {};
      transactionsState.bumpFeeAttempts[e.md.id] = e.xhr;
    },

    onBumpFeeSuccess(e) {
      app.walletBalances.get(this.activeCoin).set({
        confirmed: e.data.confirmed,
        unconfirmed: e.data.unconfirmed,
      });

      const transactionsState = this.transactionsState[this.activeCoin];
      transactionsState.cl.add(
        {
          value: e.data.amount * -1,
          txid: e.data.txid,
          timestamp: e.data.timestamp,
          address: e.data.address,
          memo: e.data.memo,
        },
        {
          parse: true,
          at: 0,
        }
      );
      this.updateTransactionsCount(this.activeCoin);
    },

    onTransactionsVwPostInit() {
      this.transactionsState[this.activeCoin].needsFetch = false;
    },

    reRenderTransactionsVw() {
      console.log('reRenderTransactionsVw triggered')
      this.showTransactionsVw = false;

      this.$nextTick(() => {
        this.showTransactionsVw = true;
      });
    },

    updateTransactionsCount(coinType = this.activeCoin) {
      this.checkCoinType(coinType);

      const transactionsState = this.transactionsState[coinType] || {};
      const cl = transactionsState && transactionsState.cl;

      let count = cl ? cl.length : 0;
      if (coinType == this.activeCoin) {
        const newTxs = this.$refs.transactionsVw ? this.$refs.transactionsVw.newTransactionsTXs : {};

        count += newTxs ? newTxs.size : 0;
      }

      this.setCountAtFirstFetch(count, coinType);
    },

    setCountAtFirstFetch(count, coinType = this.activeCoin) {
      if (typeof count !== 'number') {
        throw new Error('Please provide a count as a number.');
      }

      this.checkCoinType(coinType);
      console.log('this.transactionsState[coinType]', this.transactionsState[coinType]);
      if (!this.transactionsState[coinType] || this.transactionsState[coinType].countAtFirstFetch !== count) {
        this.transactionsState[coinType] = this.transactionsState[coinType] || {};
        this.transactionsState[coinType].countAtFirstFetch = count;
      }
    },
  },
};
</script>
<style lang="scss" scoped></style>
