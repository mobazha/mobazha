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
              <a class="jsModalClose tx6 txU" @click.stop="onClose">{{ ob.polyT('wallet.closeLink') }}</a>
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
                      :options="{ ...coin, active: coin.code === activeCoin }"
                      @click="coinSelected(coin)"
                    />
                  </ul>
                </div>
                <div v-if="false" class="js-cryptoListingsTeaser border clrP clrBr clrSh3">
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
                <template v-if="activeCoin">
                  <div class="js-coinStatsContainer">
                    <CoinStats :options="coinStatsState" />
                  </div>
                  <div>
                    <div class="flexColWide clrP clrSh3">
                      <div class="js-sendReceiveNavContainer rowMd"></div>
                      <SendReceiveNav class="rowMd" :tabActive="tabActive" :activeCoin="activeCoin" @changeTab="changeTab" />
                      <div class="js-sendReceiveContainer sendReceiveContainer clrP">
                        <SendMoney v-if="tabActive === 'send'" ref="sendeMoneyVw" :key="activeCoin" :options="{ coinType: activeCoin }" />
                        <ReceiveMoney
                          v-if="tabActive === 'receive'"
                          ref="receiveMoneyVw"
                          :key="activeCoin"
                          :coinType="activeCoin"
                        />
                        <External v-if="tabActive === 'external' && activeCoin !== 'MATICMBZ'" ref="external" :key="activeCoin" :coinType="activeCoin" />
                      </div>
                    </div>
                  </div>
                  <div class="clrP clrSh3 posR">
                    <div class="js-transactionsContainer">
                      <TransactionsVw
                        ref="transactionsVw"
                        v-if="activeCoin"
                        :options="transactionViewOptions"
                        @transactionsUpdate="onTransactionsUpdate"
                        @bumpFeeAttempt="onBumpFeeAttempt"
                        @bumpFeeSuccess="onBumpFeeSuccess"
                        @postInit="onTransactionsVwPostInit"
                        :key="transactionsVwKey"
                      />
                    </div>
                    <div class="js-reloadTransactionsContainer reloadTransactions">
                      <ReloadTransactions :options="{ initialState: { coinType: activeCoin } }" :key="transactionsVwKey" />
                    </div>
                  </div>
                </template>

                <template v-else>
                  <div class="clrP clrSh3 clrBr border zeroSupportedCurs">
                    <div class="center">{{ ob.polyT('wallet.zeroSupportedCurs') }}</div>
                  </div>
                </template>
              </div>
            </div>
          </div>
        </div>
      </template>
    </BaseModal>
    <Teleport to="#js-vueModal">
      <EditListing v-if="showEditListing"
        :bb="() => {
          return {
            model: editListingModel,
          };
        }"
        @close="closeEditListingModal"
      />
		</Teleport>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import { myGet, myPost } from '../../../api/api';
import bigNumber from 'bignumber.js';
import { isSupportedWalletCur, ensureMainnetCode, supportedWalletCurs } from '../../../../backbone/data/walletCurrencies';
import defaultSearchProviders from '../../../../backbone/data/defaultSearchProviders';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import app from '../../../../backbone/app';
import Transactions from '../../../../backbone/collections/wallet/Transactions';
import Listing from '../../../../backbone/models/listing/Listing';
import CoinNavItem from './CoinNavItem.vue';
import CoinStats from './CoinStats.vue';
import SendReceiveNav from './SendReceiveNav.vue';
import SendMoney from './SendMoney.vue';
import ReceiveMoney from './ReceiveMoney.vue';
import External from './External.vue';
import TransactionsVw from './transactions/Transactions.vue';
import ReloadTransactions from './ReloadTransactions.vue';
import CryptoListingsTeaser from './CryptoListingsTeaser.vue';
import EditListing from '@/views/modals/editListing/EditListing.vue';

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
    External,
    EditListing,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      balanceKey: 0,
      transactionsVwKey: 0,

      activeCoin: '',
      viewCryptoListingsUrl: '',
      tabActive: 'send',

      transactionsCount: 0,

      transactionsState: {},

      showEditListing: false,
      editListingModel: {},
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    if (this.tabActive === 'send') {
      if (this.$refs.sendeMoneyVw) this.$refs.sendeMoneyVw.focusAddress();
    }
  },
  unmounted() {
    Object.keys(this.transactionsState).forEach((coinType) => {
      if (this.transactionsState[coinType] && typeof this.transactionsState[coinType].bumpFeeAttempts === 'object') {
        Object.keys(this.transactionsState[coinType].bumpFeeAttempts).forEach((txId) => this.transactionsState[coinType].bumpFeeAttempts[txId].abort());
      }
    });
    this.popInTimeouts.forEach((timeout) => clearTimeout(timeout));
  },
  watch: {
    activeCoin(coin, oldVal) {
      myPost(app.getServerUrl(`wallet/status/${coin}`));

      if (this.tabActive === 'send' && !(app.walletBalances.get(coin) && app.walletBalances.get(coin).get('confirmed'))) {
        this.tabActive = 'receive';
      } else {
        this.tabActive = 'send';
      }

      this.transactionsVwKey += 1;
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
      let access = this.balanceKey;

      const { activeCoin } = this;

      const balances = app.walletBalances.toJSON();
      const balance = balances.find((item) => item.code === activeCoin);
      return {
        cryptoCur: activeCoin && ensureMainnetCode(activeCoin),
        confirmed: balance && balance.confirmed,
        unconfirmed: balance && balance.unconfirmed,
        transactionCount: this.transactionsCount,
      };
    },

    transactionViewOptions() {
      let coin = this.activeCoin;
      const transactionsState = this.transactionsState[coin] || { needsFetch: true };

      let cl = transactionsState.cl;
      if (!cl) {
        cl = new Transactions([], { coinType: coin });
        transactionsState.cl = cl;
      }

      return {
        collection: cl,
        // $scrollContainer: this.$el,
        fetchOnInit: transactionsState.needsFetch,
        countAtFirstFetch: transactionsState.countAtFirstFetch,
        bumpFeeXhrs: transactionsState.bumpFeeAttempts || undefined,
      };
    },

    navCoins() {
      let access = this.balanceKey;

      let supportedCoins = this.supportedCoins();
      const balances = app.walletBalances.toJSON();

      return supportedCoins.map((coin) => {
        const balanceMd = balances.find((item) => item.code === coin);
        return {
          active: coin === this.activeCoin,
          code: coin,
          name: app.polyglot.t(`cryptoCurrencies.${coin}`, { _: coin }),
          balance: balanceMd.confirmed,
          clientSupported: true,
        };
      });
    },
  },
  methods: {
    supportedCoins() {
      return supportedWalletCurs({ clientSupported: false }).sort((a, b) => {
        const aSortVal = app.polyglot.t(`cryptoCurrencies.${a}`, { _: a });
        const bSortVal = app.polyglot.t(`cryptoCurrencies.${b}`, { _: b });

        return aSortVal.localeCompare(bSortVal, app.localSettings.standardizedTranslatedLang(), { sensitivity: 'base' });
      });
    },

    loadData() {
      let supportedCoins = this.supportedCoins();
      let initialActiveCoin = supportedCoins.find((coin) => isSupportedWalletCur(coin)) || null;

      // If at this point the initialActiveCoin and consequently this.activeCoin
      // are null, it indicates that none of the wallet currencies are supported by
      // this client.

      (this.tabActive = !!(app.walletBalances.get(initialActiveCoin) && app.walletBalances.get(initialActiveCoin).get('confirmed')) ? 'send' : 'receive'),
        (this.activeCoin = initialActiveCoin);

      // The majority of the TransactionsVw state is managed within the component, but
      // some of it we'll manage so as you nav from coin to coin, certain state is maintained.
      this.transactionsState = supportedCoins.reduce((acc, coin) => {
        acc[coin] = { needsFetch: true };
        return acc;
      }, {});
      this.popInTimeouts = [];

      const ob1ProviderData = defaultSearchProviders.find((provider) => provider.id === 'mbz');
      this.viewCryptoListingsUrl = ob1ProviderData ? `#search?providerQ=${ob1ProviderData.listings}?type=cryptocurrency` : null;

      const serverSocket = getSocket();

      app.walletBalances.on('change', () => this.balanceKey += 1);

      if (initialActiveCoin && serverSocket) {
        this.listenTo(serverSocket, 'message', (e) => {
          if (e.jsonData.wallet && e.jsonData.wallet.transaction) {
            this.transactionsVwKey += 1;

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
              }
            }
          }
        });
      }
    },

    onClose() {
      this.$emit('close');
    },

    coinSelected(coin) {
      if (!coin.active && coin.clientSupported) {
        this.activeCoin = coin.code;
      }
    },

    onClickCreateListing() {
      this.editListingModel = new Listing({
        metadata: {
          contractType: 'CRYPTOCURRENCY',
        },
      });

      recordEvent('Listing_NewCryptoFromWallet');

      this.showEditListing = true;
    },

    closeEditListingModal() {
      this.showEditListing = false;
    },

    onClickViewCryptoListings() {
      recordEvent('Wallet_ViewCryptoListings');
    },

    changeTab(val) {
      this.tabActive = val;
    },

    checkCoinType(coinType) {
      if (typeof coinType !== 'string' || !coinType) {
        throw new Error('Please provide the coinType as a string.');
      }
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

    onTransactionsUpdate() {
      let coinType = this.activeCoin;
      const transactionsState = this.transactionsState[coinType] || {};
      const cl = transactionsState && transactionsState.cl;
      const newTxs = this.$refs.transactionsVw ? this.$refs.transactionsVw.newTransactionsTXs : {};

      this.transactionsCount = (cl ? cl.length : 0) + (newTxs ? newTxs?.size ?? 0 : 0);
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

      if (coinType === this.activeCoin) {
        this.transactionsCount = count;
      }
    },
  },
};
</script>
<style lang="scss" scoped></style>
