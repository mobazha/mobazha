<template>
  <div class="transactionsTableWrap">
    <div v-if="ob.isFetching" class="center">
      <SpinnerSVG className="spinnerMd" />
    </div>

    <div v-else-if="ob.fetchFailed" class="center txCtr tx4">
      <div :class="`txB ${ob.initialFetchErrorMessage ? 'rowTn' : 'row'}`">{{ ob.polyT(`transactions.${ob.type}.unableToFetch`) }}</div>
      <div v-if="ob.fetchError" class="row">{{ ob.fetchError }}</div>

      <a class="btn clrP clrBr clrSh2" @click="onClickRetryFetch">{{ ob.polyT(`transactions.transactionsTable.btnRetryFetch`) }}</a>
    </div>

    <template v-else-if="ob.transactions.length">
      <table class="js-transactionsTable transactionsTable clrBr clrP row">
        <tr>
          <th class="clrBr">{{ ob.polyT('transactions.transactionsTable.headers.orderID') }}</th>
          <th class="clrBr">
            <a class="js-dateHeader dateHeader clrT"
              >{{ ob.polyT('transactions.transactionsTable.headers.date') }}
              <div class="sortIcon hide"></div
            ></a>
          </th>
          <th v-if="ob.type !== 'cases'" class="clrBr">{{ ob.polyT('transactions.transactionsTable.headers.listing') }}</th>
          <th v-if="ob.type === 'sales'" class="clrBr">{{ ob.polyT('transactions.transactionsTable.headers.buyer') }}</th>
          <th v-else class="clrBr">{{ ob.polyT('transactions.transactionsTable.headers.vendor') }}</th>

          <th v-if="ob.type === 'cases'" class="clrBr">{{ ob.polyT('transactions.transactionsTable.headers.buyer') }}</th>

          <th class="clrBr priceHeader">{{ ob.polyT('transactions.transactionsTable.headers.total') }}</th>
          <th class="clrBr">{{ ob.polyT('transactions.transactionsTable.headers.status') }}</th>
        </tr>

        <Row
          v-for="transaction in transToRender"
          :key="transaction.id"
          :ref="(el) => views[transaction.id] = el"
          :options="{
            type: this.type,
            initialState: {
              acceptOrderInProgress: acceptingOrder(transaction.id),
              rejectOrderInProgress: rejectingOrder(transaction.id),
              cancelOrderInProgress: cancelingOrder(transaction.id),
            },
          }"
          :bb="function() {
            return {
              model: transaction,
            };
          }"
          @clickAcceptOrder="onClickAcceptOrder(transaction.id)"
          @clickRejectOrder="onClickRejectOrder(transaction.id)"
          @clickCancelOrder="onClickCancelOrder(transaction.id)"
          @clickRow="onClickRow(transaction.id)"
        />
      </table>
      <div class="js-pageControlsContainer"></div>
      <PageControls
        :options="{
          start: pageStartIndex + 1,
          end: pageEnd,
          total: queryTotal
        }"
        @clickNext="onClickNextPage"
        @clickPrev="onClickPrevPage"
      />
    </template>

    <div v-else class="contentBox clrP clrBr noResultsWrap">
      <div class="center">{{ ob.polyT(`transactions.${ob.type}.noResults`) }}</div>
    </div>
  </div>
</template>

<script>
/*
  This table is re-used for Sales, Purchases and Cases.
*/

import $ from 'jquery';
import _ from 'underscore';
import app from '../../../../backbone/app';
import { getContentFrame } from '../../../../backbone/utils/selectors';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import { acceptingOrder, acceptOrder, rejectingOrder, rejectOrder, cancelingOrder, cancelOrder, events as orderEvents } from '../../../../backbone/utils/order';
import { getCachedProfiles } from '../../../../backbone/models/profile/Profile';
import Row from './Row.vue';

export default {
  components: {
    Row,
  },
  props: {
    options: {
      type: Object,
      default: {
        type: '',
      },
    },
    bb: Function,
  },
  data() {
    return {
      type: 'sales',
      curPage: 1,
      queryTotal: 0,

      transactionsPerPage: 20,

      filterParams: {},

      _state: {
        isFetching: false,
        fetchError: '',
      },

      views: {},
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
  },
  unmounted() {
    if (this.avatarPost) this.avatarPost.abort();
    if (this.transactionsFetch) this.transactionsFetch.abort();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        type: this.type,
        transactions: this.collection,
        ...this._state,
      };
    },
    pageStartIndex() {
      return (this.curPage - 1) * this.transactionsPerPage;
    },
    pageEnd() {
      const onLastPage = this.curPage > this.collection.length / this.transactionsPerPage;
      let end = this.curPage * this.transactionsPerPage;

      if (onLastPage) {
        end = this.collection.length;
      }
      return end;
    },
    transToRender() {
      if (!this.collection || this.collection.length == 0) {
        return [];
      }
      // The collection contains all pages we've fetched, but we'll slice it and
      // only render the current page.
      const startIndex = this.pageStartIndex;
      return this.collection.slice(startIndex, startIndex + this.transactionsPerPage);
    },
    /*
     * Index the Row Views by Vendor and/or Buyer ID
     * so they could be easily retreived by the respective identifier.
     */
     indexedViews() {
      let indexedViews = {
        byVendor: {},
        byBuyer: {},
      };
      if (!this.views.length) {
        return indexedViews;
      }

      this.views.forEach((view) => {
        if (view == null) return;

        const vendorID = view.model.get('vendorID');
        const buyerID = view.model.get('buyerID');

        if (vendorID) {
          if (!indexedViews.byVendor[vendorID]) {
            indexedViews.byVendor[vendorID] = [];
          };
          indexedViews.byVendor[vendorID].push(view);
        }

        if (buyerID) {
          if (!indexedViews.byBuyer[buyerID]) {
            indexedViews.byBuyer[buyerID] = [];
          }
          indexedViews.byBuyer[buyerID].push(view);
        }
      });
      return indexedViews;
    },
  },
  watch: {
    filterParams(newVal, oldVal) {
      if (!_.isEqual(newVal, oldVal)) {
        this.setFilterOnRoute();

        this.collection.reset();
        this.fetchTransactions(1);
      }
    },

    transToRender (models) {
      if (models && models.length > 0) {
        this.getAvatars(models);
      }
    }
  },
  methods: {
    acceptingOrder,
    rejectingOrder,
    cancelingOrder,
    loadData(options = {}) {
      const types = ['sales', 'purchases', 'cases'];
      const opts = {
        initialState: {
          isFetching: false,
          fetchError: '',
        },
        type: 'sales',
        ...options,
      };

      if (types.indexOf(opts.type) === -1) {
        throw new Error('Please provide a valid type.');
      }

      this.baseInit(opts);

      if (!this.collection) {
        throw new Error('Please provide a collection');
      }

      this.type = opts.type;
      this.curPage = 1;
      this.queryTotal = 0;

      const socket = getSocket();

      if (socket) {
        this.listenTo(socket, 'message', this.onSocketMessage);
      }

      this.listenTo(orderEvents, 'rejectingOrder', this.onRejectingOrder);
      this.listenTo(orderEvents, 'rejectOrderComplete, rejectOrderFail', this.onRejectOrderAlways);
      this.listenTo(orderEvents, 'rejectOrderComplete', this.onRejectOrderComplete);
      this.listenTo(orderEvents, 'acceptingOrder', this.onAcceptingOrder);
      this.listenTo(orderEvents, 'acceptOrderComplete, acceptOrderFail', this.onAcceptOrderAlways);
      this.listenTo(orderEvents, 'acceptOrderComplete', this.onAcceptOrderComplete);
      this.listenTo(orderEvents, 'cancelingOrder', this.onCancelingOrder);
      this.listenTo(orderEvents, 'cancelOrderComplete, cancelOrderFail', this.onCancelOrderAlways);
      this.listenTo(orderEvents, 'cancelOrderComplete', this.onCancelOrderComplete);
    },

    onSocketMessage(e) {
      if (e.jsonData.message) {
        // If a chat message comes in for a transaction in our list,
        // we'll update the unread count.
        const transaction = this.collection.get(e.jsonData.message.subject);

        if (transaction) {
          const count = transaction.get('unreadChatMessages');
          transaction.set({
            unreadChatMessages: count + 1,
            read: false,
          });
        }
      }
    },

    onClickRetryFetch() {
      this.fetchTransactions();
    },

    onClickRejectOrder(txid) {
      rejectOrder(txid);
    },

    onRejectingOrder(e) {
      const view = this.views[e.id];

      if (view) {
        view.setState({
          rejectOrderInProgress: true,
        });
      }
    },

    onRejectOrderAlways(e) {
      const view = this.views[e.id];

      if (view) {
        view.setState({
          rejectOrderInProgress: false,
        });
      }
    },

    onRejectOrderComplete(e) {
      const view = this.views[e.id];

      if (view) {
        view.model.set('state', 'DECLINED');
      }
    },

    onClickAcceptOrder(txid) {
      acceptOrder(txid);
    },

    onAcceptingOrder(e) {
      const view = this.views[e.id];

      if (view) {
        view.setState({
          acceptOrderInProgress: true,
        });
      }
    },

    onAcceptOrderAlways(e) {
      const view = this.views[e.id];

      if (view) {
        view.setState({
          acceptOrderInProgress: false,
        });
      }
    },

    onAcceptOrderComplete(e) {
      const view = this.views[e.id];

      if (view) {
        view.model.set('state', 'AWAITING_FULFILLMENT');
      }
    },

    onClickCancelOrder(txid) {
      cancelOrder(txid);
    },

    onCancelingOrder(e) {
      const view = this.views[e.id];

      if (view) {
        view.setState({
          cancelOrderInProgress: true,
        });
      }
    },

    onCancelOrderAlways(e) {
      const view = this.views[e.id];

      if (view) {
        view.setState({
          cancelOrderInProgress: false,
        });
      }
    },

    onCancelOrderComplete(e) {
      const view = this.views[e.id];

      if (view) {
        view.model.set('state', 'CANCELED');
      }
    },

    onClickRow(orderID) {
      let type = 'sale';

      if (this.type === 'purchases') {
        type = 'purchase';
      } else if (this.type === 'cases') {
        type = 'case';
      }
      this.$emit('clickRow', orderID, type);
    },

    onClickNextPage() {
      this.fetchTransactions((this.curPage += 1));
    },

    onClickPrevPage() {
      this.fetchTransactions((this.curPage -= 1));
    },

    getAvatars(models = []) {
      const profilesToFetch = [];

      models.forEach((md) => {
        const vendorID = md.get('vendorID');
        const buyerID = md.get('buyerID');

        if (vendorID) {
          profilesToFetch.push(vendorID);
        }

        if (buyerID) {
          profilesToFetch.push(buyerID);
        }
      });

      if (profilesToFetch.length) {
        getCachedProfiles(profilesToFetch).forEach((profilePromise) => {
          profilePromise.done((profile) => {
            const flatProfile = profile.toJSON();
            const vendorViews = this.indexedViews.byVendor[flatProfile.peerID] || [];
            const buyerViews = this.indexedViews.byBuyer[flatProfile.peerID] || [];

            vendorViews.forEach((view) => {
              view.setState({ vendorAvatarHashes: flatProfile.avatarHashes });
              view.model.set({ vendorHandle: flatProfile.handle });
            });

            buyerViews.forEach((view) => {
              view.setState({ buyerAvatarHashes: flatProfile.avatarHashes });
              view.model.set({ buyerHandle: flatProfile.handle });
            });
          });
        });
      }
    },

    setFilterOnRoute(filter = this.filterParams) {
      const queryFilter = {
        ...filter,
        // Joining with dashes instead of commas because commas
        // look really bizarre when encode in a query string.
        states: Array.isArray(filter.states) ? filter.states.join('-') : '',
      };

      if (!queryFilter.states) {
        delete queryFilter.states;
      }

      if (queryFilter.search === '') {
        delete queryFilter.search;
      }

      let baseRoute = location.hash.split('?')[0];
      baseRoute = baseRoute.startsWith('#ob://') ? baseRoute.slice(6) : baseRoute.slice(1);

      app.router.navigate(`${baseRoute}?${$.param(queryFilter)}`, { replace: true });
    },

    fetchTransactions(page = this.curPage) {
      const filterParams = this.filterParams;

      if (typeof page !== 'number') {
        throw new Error('Please provide a page number to fetch.');
      }

      if (page < 1) {
        throw new Error('Please provide a page number greater than zero.');
      }

      this.curPage = page;

      if (this.transactionsFetch) this.transactionsFetch.abort();

      const fetchParams = {
        limit: this.transactionsPerPage,
        ...filterParams,
        sortByAscending: ['UNREAD', 'DATE_ASC'].indexOf(filterParams.sortBy) === -1,
        sortByRead: filterParams.sortBy === 'UNREAD',
        exclude: this.collection.map((md) => md.id),
      };

      delete fetchParams.sortBy;
      let havePage = false;

      if (this.collection.length > (page - 1) * this.transactionsPerPage) {
        // we already have the page
        havePage = true;
        getContentFrame()[0].scrollTop = 0;
      } else if (this.collection.length < (page - 1) * this.transactionsPerPage) {
        // You cannot fetch a page unless you have its previous page. The api
        // requires the ID of the last transaction in the previous page.
        throw new Error('Cannot fetch page. Do no have the previous pages.');
      } else if (this.collection.length) {
        fetchParams.offsetID = this.collection.at(this.collection.length - 1).id;
      }

      if (havePage) return;

      this.transactionsFetch = this.collection.fetch({
        data: fetchParams,
        remove: false,
      });

      this.transactionsFetch
        .fail((jqXhr) => {
          if (jqXhr.statusText === 'abort') return;

          let fetchError = '';

          if (jqXhr.responseJSON && jqXhr.responseJSON.reason) {
            fetchError = jqXhr.responseJSON.reason;
          }

          this.setState({
            isFetching: false,
            fetchFailed: true,
            fetchError,
          });
        })
        .done((data, textStatus, jqXhr) => {
          if (jqXhr.statusText === 'abort') return;

          this.queryTotal = data.queryCount;

          this.setState({
            isFetching: false,
          });
        });

      this.setState({
        isFetching: true,
        fetchFailed: false,
        fetchError: '',
      });
    },
  },
};
</script>
<style lang="scss" scoped></style>
