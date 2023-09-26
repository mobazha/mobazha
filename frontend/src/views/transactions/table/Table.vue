<template>
  <div class="transactionsTableWrap">
    <div v-if="ob.isFetching">
      <div class="center">
        <SpinnerSVG className="spinnerMd" />
      </div>
    </div>

    <div v-else-if="ob.fetchFailed">
      <div class="center txCtr tx4">
        <div class="txB <% print(ob.initialFetchErrorMessage ? 'rowTn' : 'row') %>">{{ ob.polyT(`transactions.${ob.type}.unableToFetch`) }}</div>
        <div v-if="ob.fetchError" class="row">{{ ob.fetchError }}</div>

        <a class="btn clrP clrBr clrSh2" @click="onClickRetryFetch">{{ ob.polyT(`transactions.transactionsTable.btnRetryFetch`) }}</a>
      </div>
    </div>

    <div v-else>
      <div v-if="ob.transactions.length">
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
            v-for="(transaction, key) in transToRender"
            :key="key"
            ref="views"
            :options="{
              model: transaction,
              type: this.type,
              initialState: {
                acceptOrderInProgress: acceptingOrder(transaction.id),
                rejectOrderInProgress: rejectingOrder(transaction.id),
                cancelOrderInProgress: cancelingOrder(transaction.id),
              },
            }"
            @clickAcceptOrder="onClickAcceptOrder(transaction.id)"
            @clickRejectOrder="onClickRejectOrder(transaction.id)"
            @clickCancelOrder="onClickCancelOrder(transaction.id)"
            @clickRow="onClickRow(transaction.id)"
          />
        </table>
        <div class="js-pageControlsContainer"></div>
        <PageControls
          :options="{ initialState: { start: pageStartIndex + 1, pageEnd, total: queryTotal } }"
          @clickNext="onClickNextPage"
          @clickPrev="onClickPrevPage"
        />
      </div>

      <div v-else>
        <div class="contentBox clrP clrBr noResultsWrap">
          <div class="center">{{ ob.polyT(`transactions.${ob.type}.noResults`) }}</div>
        </div>
      </div>
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
  mixins: [],
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      transactionsPerPage: 20,

      filterParams: {},
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.$props.options);
  },
  mounted() {
    this.onAttach();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        type: this.type,
        transactions: this.collection.toJSON(),
        ...this._state,
      };
    },
    transactions() {
      return this.collection.toJSON();
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
      console.log();
      if (!this.collection || this.collection.length == 0) {
        return [];
      }
      // The collection contains all pages we've fetched, but we'll slice it and
      // only render the current page.
      const startIndex = this.pageStartIndex;
      return this.collection.slice(startIndex, startIndex + this.transactionsPerPage);
    },
  },
  watch: {
    filterParams(newVal, oldVal) {
      if (!_.isEqual(newVal, oldVal)) {
        this.collection.reset();
        this.fetchTransactions(1, newVal);
      }
    },

    // transToRender () {
    //   this.indexRowViews();
    //   this.getAvatars(this.transToRender); // be sure to get avatars *after* indexRowViews()
    // }
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

      if (typeof opts.openOrder !== 'function') {
        throw new Error('Please provide a function to open the order detail modal.');
      }
      _.extend(this, opts);
      this.setState(opts.initialState || {});

      if (!this.collection) {
        throw new Error('Please provide a collection');
      }

      this.type = opts.type;
      this.curPage = 1;
      this.queryTotal = null;

      const socket = getSocket();

      if (socket) {
        this.listenTo(socket, 'message', this.onSocketMessage);
      }

      if (this.options.openedOrderModal) {
        this.bindOrderDetailEvents(this.options.openedOrderModal);
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
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.setState({
          rejectOrderInProgress: true,
        });
      }
    },

    onRejectOrderAlways(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.setState({
          rejectOrderInProgress: false,
        });
      }
    },

    onRejectOrderComplete(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.model.set('state', 'DECLINED');
      }
    },

    onClickAcceptOrder(txid) {
      acceptOrder(txid);
    },

    onAcceptingOrder(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.setState({
          acceptOrderInProgress: true,
        });
      }
    },

    onAcceptOrderAlways(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.setState({
          acceptOrderInProgress: false,
        });
      }
    },

    onAcceptOrderComplete(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.model.set('state', 'AWAITING_FULFILLMENT');
      }
    },

    onClickCancelOrder(txid) {
      cancelOrder(txid);
    },

    onCancelingOrder(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.setState({
          cancelOrderInProgress: true,
        });
      }
    },

    onCancelOrderAlways(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.setState({
          cancelOrderInProgress: false,
        });
      }
    },

    onCancelOrderComplete(e) {
      const view = this.indexedViews.byOrder[e.id];

      if (view) {
        view.model.set('state', 'CANCELED');
      }
    },

    onClickRow(txid) {
      let type = 'sale';

      if (this.type === 'purchases') {
        type = 'purchase';
      } else if (this.type === 'cases') {
        type = 'case';
      }

      const orderDetail = this.options.openOrder(txid, type);
      this.bindOrderDetailEvents(orderDetail);
    },

    bindOrderDetailEvents(orderDetail) {
      this.listenTo(orderDetail.model, 'sync', () => {
        const transaction = this.collection.get(orderDetail.model.id);

        if (transaction) {
          transaction.set('read', true);
        }
      });

      this.listenTo(orderDetail, 'convoMarkedAsRead', () => {
        const transaction = this.collection.get(orderDetail.model.id);

        if (transaction) {
          transaction.set({
            unreadChatMessages: 0,
            read: true,
          });
        }
      });
    },

    onClickNextPage() {
      this.fetchTransactions((this.curPage += 1));
    },

    onClickPrevPage() {
      this.fetchTransactions((this.curPage -= 1));
    },

    onAttach() {
      this.setFilterOnRoute();
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

    /*
     * Index the Row Views by Vendor and/or Buyer ID as well as orderID
     * so they could be easily retreived by the respective identifier.
     */
    indexRowViews() {
      this.indexedViews = {
        byVendor: {},
        byBuyer: {},
        byOrder: {},
      };

      this.refs.views.forEach((view) => {
        const vendorID = view.model.get('vendorID');
        const buyerID = view.model.get('buyerID');

        if (vendorID) {
          this.indexedViews.byVendor[vendorID] = this.indexedViews.byVendor[vendorID] || [];
          this.indexedViews.byVendor[vendorID].push(view);
        }

        if (buyerID) {
          this.indexedViews.byBuyer[buyerID] = this.indexedViews.byBuyer[buyerID] || [];
          this.indexedViews.byBuyer[buyerID].push(view);
        }

        this.indexedViews.byOrder[view.model.id] = view;
      });
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

    fetchTransactions(page = this.curPage, filterParams = this.filterParams) {
      if (typeof page !== 'number') {
        throw new Error('Please provide a page number to fetch.');
      }

      if (page < 1) {
        throw new Error('Please provide a page number greater than zero.');
      }

      this.curPage = page;
      this.filterParams = filterParams;
      this.setFilterOnRoute();

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

    remove() {
      if (this.avatarPost) this.avatarPost.abort();
      if (this.transactionsFetch) this.transactionsFetch.abort();
    },
  },
};
</script>
<style lang="scss" scoped></style>
