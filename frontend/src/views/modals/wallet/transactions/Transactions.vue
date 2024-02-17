<template>
  <div class="walletTransactions">
    <h2 class="h5">{{ ob.polyT('wallet.transactions.header') }}</h2>
    <div class="posR">
      <PopInMessage
        v-if="showNewTxPopup"
        :class="`popInMessageHolder js-popInMessages ${notFixedMessages ? 'notFixed' : ''}`"
        :options="{ messageText: ob.polyT('wallet.transactions.newTransactionsPopin', { smart_count: newTransactionsTXs.size }) }"
        @clickRefresh="refreshTransactions()"
        @clickDismiss="this.showNewTxPopup = false"
      />
      <div v-for="(model, key) in collection.models" :key="key" class="js-transactionListContainer borderStacked clrBr padMdKids">
        <Transaction
          ref="transaction"
          :options="{
            coinType: this.coinType,
            bumpFeeXhr: (this.bumpFeeXhrs && this.bumpFeeXhrs[model.id]) || undefined,
          }"
          :bb="function() {
            return {
              model: model,
            };
          }"
          @bumpFeeAttempt="onBumpFeeAttempt"
          @bumpFeeSuccess="onBumpFeeSuccess"
        />
      </div>
    </div>
    <TransactionFetchState
      :options="{
        isFetching,
        fetchFailed,
        fetchErrorMessage,
        transactionsPresent: !!options.collection.length,
      }"
      @clickRetryFetch="clickRetryFetch"
    />
    <div class="js-transactionEmptyState"></div>
    <template v-if="!ob.transactions.length && !isFetching && !ob.fetchFailed">
      <div class="center txCtr tx5 clrT2">
        {{ ob.polyT('wallet.transactions.noTransactionsPlaceholder', { coinType: ob.polyT(`cryptoCurrencies.${ob.coinType}`, { _: ob.coinType }) }) }}
      </div>
    </template>
    <Teleport to="#js-vueModal">
      <Settings v-if="showSettings" :options="{ initialTab: 'Advanced', scrollTo: '.js-backupWalletSection', }" @close="closeSettings" />
    </Teleport>
  </div>
</template>

<script>
import _ from 'underscore';
import app from '../../../../../backbone/app';
import { isScrolledIntoView } from '../../../../../backbone/utils/dom';
import { getSocket, getCurrentConnection } from '../../../../../backbone/utils/serverConnect';
import { openSimpleMessage } from '../../../../../backbone/views/modals/SimpleMessage';
import Transaction from './Transaction.vue';
import TransactionFetchState from './TransactionFetchState.vue';
import Settings from '@/views/modals/settings/Settings.vue';

export default {
  components: {
    Transaction,
    TransactionFetchState,
    Settings,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      transactionsPerFetch: 25,
      transactionsFetch: undefined,

      transactionViews: [],
      fetchFailed: false,
      fetchErrorMessage: '',
      isFetching: false,
      newTransactionsTXs: new Set(),
      popInTimeouts: [],
      coinType: '',

      showNewTxPopup: false,

      _state: {
        // If not fetching on init, you may want to pass in the count of transactions
        // that were returned by the first fetch which is used to determine if all the
        // pages have been fetched.
        countAtFirstFetch: undefined,
        // If there are any existing bump fee attempts that you want shuttled into the
        // individual transaction views, please provide an indexed object (indexed by txid)
        // of them here.
        bumpFeeXhrs: undefined,
      },

      showSettings: false,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.$emit('postInit');

    window.addEventListener('scroll', this.throttledOnScroll);
  },
  beforeRouteLeave() {
    if (this.transactionsFetch) this.transactionsFetch.abort();
    this.popInTimeouts.forEach((timeout) => clearTimeout(timeout));
  },
  unmounted() {
    window.removeEventListener('scroll', this.throttledOnScroll);
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        transactions: this.collection,
        fetchFailed: this.fetchFailed,
        coinType: this.coinType,
      };
    },
    allLoaded() {
      return this.collection.length >= this.countAtFirstFetch;
    },
    notFixedMessages() {
      return this.scrollTop < 515;
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        fetchOnInit: true,
        // If not fetching on init, you may want to pass in the count of transactions
        // that were returned by the first fetch which is used to determine if all the
        // pages have been fetched.
        countAtFirstFetch: undefined,
        // If there are any existing bump fee attempts that you want shuttled into the
        // individual transaction views, please provide an indexed object (indexed by txid)
        // of them here.
        bumpFeeXhrs: undefined,
        ...options,
      };

      this.baseInit(opts);

      if (!this.collection) {
        throw new Error('Please provide a Transactions collection.');
      }

      // if (!opts.$scrollContainer || !opts.$scrollContainer.length) {
      //   throw new Error('Please provide a jQuery object containing the scrollable element '
      //     + 'this view is in.');
      // }
      this.coinType = this.collection.options.coinType;
      this.countAtFirstFetch = opts.countAtFirstFetch;

      const serverSocket = getSocket();

      if (serverSocket) {
        this.listenTo(serverSocket, 'message', (e) => {
          // The "walletUpdate" socket comes on a regular interval and gives us the current block
          // height which we can use to update the confirmations on a transaction.
          if (e.jsonData.walletUpdate && e.jsonData.walletUpdate[this.coinType]) {
            const walletUpdate = e.jsonData.walletUpdate[this.coinType];
            this.collection.models
              .filter((transaction) => transaction.get('height') > 0)
              .forEach((transaction) => {
                const confirmations = walletUpdate.height - transaction.get('height') + 1;
                transaction.set('confirmations', confirmations);
              });
          }
        });
      }

      // this.$scrollContainer = opts.$scrollContainer;
      this.throttledOnScroll = _.throttle(this.onScroll, 100).bind(this);

      this.refreshTransactions();
    },

    onScroll(event) {
      this.scrollTop = event.currentTarget.scrollTop;

      if (this.collection.length && !this.allLoaded) {
        // fetch next batch of transactions
        const lastTransaction = this.transactionViews[this.transactionViews.length - 1];

        if (!this.isFetching && isScrolledIntoView(lastTransaction.el)) {
          this.fetchTransactions();
        }
      }
    },

    refreshTransactions() {
      this.collection.reset();
      this.fetchTransactions();
    },

    fetchTransactions() {
      if (this.transactionsFetch && this.transactionsFetch.state() === 'pending') return;

      const fetchParams = {
        limit: this.transactionsPerFetch,
      };

      if (this.collection.length) {
        fetchParams.offsetID = this.collection.at(this.collection.length - 1).id;
      }

      this.transactionsFetch = this.collection.fetch({
        data: fetchParams,
        remove: false,
      });

      this.transactionsFetch
        .done((data) => {
          if (this.isRemoved()) return;

          this.isFetching = false;
          this.fetchFailed = false;
          this.fetchErrorMessage = '';

          this.$emit('transactionsUpdate');

          if (typeof this.countAtFirstFetch === 'undefined') {
            this.countAtFirstFetch = data.count;
          }

          if (this.collection.length) {
            const curConn = getCurrentConnection();

            if (curConn && curConn.server && !curConn.server.get('backupWalletWarned')) {
              const warning = openSimpleMessage(
                app.polyglot.t('wallet.transactions.backupWalletWarningTitle'),
                app.polyglot.t('wallet.transactions.backupWalletWarningBody', {
                  link: '<a class="js-recoverWalletSeed">' + `${app.polyglot.t('wallet.transactions.recoverySeedLink')}</a>`,
                })
              );

              warning.$el.on('click', '.js-recoverWalletSeed', () => {
                this.showSettings = true;
                warning.remove();
              });

              curConn.server.save({ backupWalletWarned: true });
            }
          }
        })
        .fail((xhr) => {
          if (this.isRemoved() || xhr.statusText === 'abort') return;

          this.$emit('transactionsUpdate');

          this.isFetching = false;
          this.fetchFailed = true;
          this.fetchErrorMessage = (xhr.responseJSON && xhr.responseJSON.reason) || '';
        });

      this.isFetching = true;
      this.transactionsPresent = !!this.collection.length;

      this.fetchFailed = false;
      this.fetchErrorMessage = '';
    },

    closeSettings() {
      this.showSettings = false;
    },

    showNewTransactionPopup() {
      this.showNewTxPopup = true;
    },

    onBumpFeeAttempt(e) {
      this.$emit('bumpFeeAttempt', e);
    },

    onBumpFeeSuccess(e) {
      this.$emit('bumpFeeSuccess', e);
    },

    clickRetryFetch() {
      // simulate some latency so if it fails again, it looks like it tried.
      this.isFetching = true;
      setTimeout(() => this.fetchTransactions(), 250);
    },
  },
};
</script>
<style lang="scss" scoped></style>
