<template>
  <div class="walletTransactions">
    <h2 class="h5">{{ ob.polyT('wallet.transactions.header') }}</h2>
    <div class="posR">
      <PopInMessage
        v-if="showNewTxPopup"
        :class="`popInMessageHolder js-popInMessages ${notFixedMessages ? 'notFixed' : ''}`"
        :options="{ messageText: ob.polyT('wallet.transactions.newTransactionsPopin', { smart_count: newTransactionsTXs.size },), }"
        @clickRefresh="refreshTransactions()"
        @clickDismiss="this.showNewTxPopup = false"
      />
      <div v-for="(model, key) in collection.models" :key="key" class="js-transactionListContainer borderStacked clrBr padMdKids">
        <Transaction
        :options="{
          model,
          coinType: this.coinType,
          bumpFeeXhr: (this.bumpFeeXhrs && this.bumpFeeXhrs[model.id]) || undefined,
        }"
        :bumpFeeAttempt="onBumpFeeAttempt"
        :bumpFeeSuccess="onBumpFeeSuccess"
         />
      </div>
    </div>
    <TransactionFetchState
      ref="transactionFetchState"
      :options="{
        initialState: {
          isFetching,
          fetchFailed,
          fetchErrorMessage,
          transactionsPresent: !!this.collection.length,
        },
      }"
      :clickRetryFetch="clickRetryFetch"
      />
    <div class="js-transactionEmptyState"></div>
    <div v-if="!ob.transactions.length && !isFetching && !ob.fetchFailed">
      <div class="center txCtr tx5 clrT2">
        {{ ob.polyT('wallet.transactions.noTransactionsPlaceholder',
          { coinType: ob.polyT(`cryptoCurrencies.${ob.coinType}`, { _: ob.coinType }) }) }}
      </div>
    </div>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import app from '../../../../../backbone/app';
import { isScrolledIntoView } from '../../../../../backbone/utils/dom';
import { getSocket, getCurrentConnection } from '../../../../../backbone/utils/serverConnect';
import { openSimpleMessage } from '../../../../../backbone/views/modals/SimpleMessage';
import { launchSettingsModal } from '../../../../../backbone/utils/modalManager';
import Transaction from './Transaction.vue';
import TransactionFetchState from './TransactionFetchState.vue';


export default {
  components: {
    Transaction,
    TransactionFetchState,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      transactionsPerFetch: 25,

      transactionViews: [],
      fetchFailed: false,
      fetchErrorMessage: '',
      newTransactionsTXs: new Set(),
      popInTimeouts: [],
      coinType: '',
      countAtFirstFetch: false,

      showNewTxPopup: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);

    window.addEventListener('scroll', this.throttledOnScroll);
  },
  mounted () {
    if (this.fetchOnInit) this.refreshTransactions();

    this.$emit('postInit');
  },
  unmounted () {
    if (this.transactionsFetch) this.transactionsFetch.abort();
    this.popInTimeouts.forEach((timeout) => clearTimeout(timeout));

    window.removeEventListener('scroll', this.throttledOnScroll);
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        transactions: this.collection.toJSON(),
        fetchFailed: this.fetchFailed,
        coinType: this.coinType,
      };
    },
    isFetching () {
      return this.transactionsFetch
        && this.transactionsFetch.state() === 'pending';
    },
    allLoaded () {
      return this.collection.length >= this.countAtFirstFetch;
    },
    notFixedMessages () {
      return this.scrollTop < 515;
    },
  },
  methods: {
    loadData (options = {}) {
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
              .filter((transaction) => (transaction.get('height') > 0))
              .forEach((transaction) => {
                const confirmations = walletUpdate.height - transaction.get('height') + 1;
                transaction.set('confirmations', confirmations);
              });
          }
        });
      }

      // this.$scrollContainer = opts.$scrollContainer;
      this.throttledOnScroll = _.throttle(this.onScroll, 100).bind(this);
    },

    onScroll (event) {
      this.scrollTop = event.currentTarget.scrollTop;

      if (this.collection.length && !this.allLoaded) {
        // fetch next batch of transactions
        const lastTransaction = this.transactionViews[this.transactionViews.length - 1];

        if (!this.isFetching && isScrolledIntoView(lastTransaction.el)) {
          this.fetchTransactions();
        }
      }
    },

    refreshTransactions () {
      this.collection.reset();
      this.fetchTransactions();
    },

    fetchTransactions () {
      if (
        this.transactionsFetch
        && this.transactionsFetch.state() === 'pending'
      ) return;

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

          this.fetchFailed = false;
          this.fetchErrorMessage = '';

          if (typeof this.countAtFirstFetch === 'undefined') {
            this.countAtFirstFetch = data.count;
          }

          if (this.collection.length) {
            this.$refs.transactionFetchState.setState({
              isFetching: false,
              fetchFailed: this.fetchFailed,
              fetchErrorMessage: this.fetchErrorMessage,
            });

            const curConn = getCurrentConnection();

            if (curConn && curConn.server && !curConn.server.get('backupWalletWarned')) {
              const warning = openSimpleMessage(
                app.polyglot.t('wallet.transactions.backupWalletWarningTitle'),
                app.polyglot.t('wallet.transactions.backupWalletWarningBody', {
                  link: '<a class="js-recoverWalletSeed">'
                    + `${app.polyglot.t('wallet.transactions.recoverySeedLink')}</a>`,
                }),
              );

              warning.$el.on('click', '.js-recoverWalletSeed', () => {
                launchSettingsModal({
                  initialTab: 'Advanced',
                  scrollTo: '.js-backupWalletSection',
                });
                warning.remove();
              });

              curConn.server.save({ backupWalletWarned: true });
            }
          }
        }).fail((xhr) => {
          if (this.isRemoved() || xhr.statusText === 'abort') return;

          this.fetchFailed = true;
          this.fetchErrorMessage = (xhr.responseJSON && xhr.responseJSON.reason) || '';

          if (this.collection.length) {
            this.$refs.transactionFetchState.setState({
              isFetching: false,
              fetchFailed: this.fetchFailed,
              fetchErrorMessage: this.fetchErrorMessage,
            });
          }
        });

      this.$refs.transactionFetchState.setState({
        isFetching: true,
        transactionsPresent: !!this.collection.length,
      });

      this.fetchFailed = false;
      this.fetchErrorMessage = '';
    },

    showNewTransactionPopup () {
      this.showNewTxPopup = true;
    },

    onBumpFeeAttempt(e) {
      this.$emit('bumpFeeAttempt', e);
    },

    onBumpFeeSuccess(e) {
      this.$emit('bumpFeeSuccess', e);
    },

    clickRetryFetch () {
      // simulate some latency so if it fails again, it looks like it tried.
      this.$refs.transactionFetchState.setState({ isFetching: true });
      setTimeout(() => this.fetchTransactions(), 250);
    },
  }
}
</script>
<style lang="scss" scoped></style>
