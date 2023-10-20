<template>
  <div :class="`userPageStore ${listingsViewType == 'list' ? 'listView' : ''}`" @scroll="onStoreListingsScroll">
    <div class="popInMessageHolder js-storePopInMessages"></div>
    <div class="userPageSearchBar flex gutterHSm" :disabled="ob.isFetching || ob.fetchFailed || !ob.listingCount">
      <div class="flexExpand">
        <div class="searchWrapper">
          <input type="text" class="ctrl searchInput clrP clrBr clrSh2" @keyup="onKeyupSearchInput(inputTerm)" :placeholder="ob.polyT('userPage.searchStore')" v-model="inputTerm" />
        </div>
      </div>
      <div>
        <div class="clrT2 clrSh2">
          <a class="iconBtn clrP clrBr toolTipNoWrap" @click="onClickToggleListGridView" :data-tip="ob.polyT('userPage.store.toolTip.contentView')">
            <i class="ion-android-apps gridViewIcon"></i>
            <i class="ion-navicon listViewIcon"></i>
          </a>
        </div>
      </div>
    </div>
    <template v-if="ob.isFetching">
      <div class="txCtr padHg contentBox clrBr clrP">
        <SpinnerSVG className="spinnerLg" />
      </div>
    </template>

    <template v-else-if="ob.fetchFailed">
      <div class="txCtr padHg contentBox clrBr clrP">
        <h3>{{ ob.polyT('userPage.store.unableToFetchListings') }}</h3>
        <p>{{ ob.fetchFailReason }}</p>
        <ProcessingButton className="btn clrP clrBr js-retryFetch" :btnText="ob.polyT('userPage.store.retryStoreFetch')" @click="onClickRetryFetch" />
      </div>
    </template>

    <template v-else-if="!ob.isFetching">
      <div class="flex">
        <div class="col storeFilters" :disabled="!ob.listingCount">
          <div :class="`js-shippingFilterContainer clrP clrBr padMd clrT clrSh2 contentBox ${!enableShipping ? 'disabled' : ''}`">
            <div class="txB rowSm">{{ ob.polyT('userPage.store.shippingFilter.heading') }}</div>
            <div class="flexVCent rowSm">
              <label class="margRSm" for="shipsToSelect">{{ ob.polyT('userPage.store.shippingFilter.shipsTo') }}:</label>
              <Select2 class="tx6 select2Small js-shipsToSelect" v-model="filter.shipsTo" style="width: 133px" id="shipsToSelect">
                <option value="any" :selected="ob.shipsToSelected === 'any'">{{ ob.polyT('userPage.store.shipsToFilterAny') }}</option>
                <option v-for="(country, j) in ob.countryList" :key="j" :value="country.dataName" :selected="country.dataName === ob.shipsToSelected">
                  {{ country.name }}
                </option>
              </Select2>
            </div>
            <div class="flexVCent rowSm">
              <input type="checkbox" id="filterFreeShipping" class="margRSm" v-model="filter.freeShipping" />
              <label for="filterFreeShipping"></label
              ><!-- label for the replacement checkbox -->
              <label class="clrE1 clrTOnEmph phraseBox" for="filterFreeShipping">{{ ob.polyT('userPage.store.freeShippingBanner') }}</label>
            </div>
          </div>
          <div class="js-catFilterContainer">
            <CategoryFilter :categories="collection.categories" :selected="filter.category" @category-change="onCategoryChange" />
          </div>
          <div class="js-typeFilterContainer">
            <TypeFilter :types="collection.types" :selected="filter.type" @type-change="onTypeChange" />
          </div>
        </div>
        <div class="col storeListings">
          <template v-if="ob.listingCount">
            <div class="row clrT tx5 flexVBot listingsHeaderRow">
              <span class="listingsCount js-listingCount" v-html="fullListingCount"></span>
              <div>
                <div class="tx6 flexVCent">
                  <label class="clrT2 marginLAuto margRSm">{{ ob.polyT('userPage.store.sortBy') }}</label>
                  <Select2 class="tx6 select2Small js-sortBySelect" :options="{ minimumResultsForSearch: -1, }" v-model="filter.sortBy" style="width: 150px">
                    <option value="PRICE_ASC" :selected="ob.filter.sortBy === 'PRICE_ASC'">{{ ob.polyT('userPage.store.sortOpts.priceAsc') }}</option>
                    <option value="PRICE_DESC" :selected="ob.filter.sortBy === 'PRICE_DESC'">{{ ob.polyT('userPage.store.sortOpts.priceDesc') }}</option>
                    <option value="NAME_ASC" :selected="ob.filter.sortBy === 'NAME_ASC'">{{ ob.polyT('userPage.store.sortOpts.nameAsc') }}</option>
                    <option value="NAME_DESC" :selected="ob.filter.sortBy === 'NAME_DESC'">{{ ob.polyT('userPage.store.sortOpts.nameDesc') }}</option>
                    <!-- <option value="RATING">Rating</option> -->
                  </Select2>
                </div>
              </div>
            </div>
            <div class="contentBox row pad clrP clrBr js-inactiveWarning" v-show="!ob.vendor">
              <span class="tx5"
                ><template v-html="ob.parseEmojis('🔒')"/> {{ ob.polyT('userPage.store.inactive') }}
                <button class="btnTxtOnly txU txUnb clrT2" @click="onClickActivateStore">${ob.polyT('userPage.store.inactiveLink')}</button>
              </span>
            </div>
            <div class="js-listingsContainer">
              <ListingsGrid
                :key="listingsGridKey"
                :viewType="listingsViewType"
                :bb="function() {
                  return {
                    collection: storeListingsCol,
                    storeOwnerProfile: model,
                  }
                }"/>
            </div>
            <div class="txCtr padGi clrP clrSh2 clrBr tx4 contentBox js-noResults" v-show="!filteredCollection.length">
              <p>{{ ob.polyT('userPage.store.noListingsFound') }}</p>
              <div class="btn clrP clrBr" @click="onClickClearSearch">{{ ob.polyT('userPage.store.btnClearSearch') }}</div>
            </div>
          </template>

          <template v-else>
            <div class="txCtr padGi tx4">{{ ob.polyT('userPage.store.noListings') }}</div>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<script>
import _ from 'underscore';
import $ from 'jquery';
import 'velocity-animate';
import 'velocity-animate/velocity.ui';
import { getTranslatedCountries } from '../../../backbone/data/countries';
import app from '../../../backbone/app';
import { convertCurrency, NoExchangeRateDataError } from '../../../backbone/utils/currency';
import { launchSettingsModal } from '../../../backbone/utils/modalManager';
import Listing from '../../../backbone/models/listing/Listing';
import Listings from '../../../backbone/collections/Listings';
import { events as listingEvents } from '../../../backbone/models/listing';
import ListingDetail from '../../../backbone/views/modals/listingDetail/Listing';

import CategoryFilter from './CategoryFilter.vue';
import TypeFilter from './TypeFilter.vue';
import ListingsGrid from './ListingsGrid.vue'
import PopInMessage, { buildRefreshAlertMessage } from '../../../backbone/views/components/PopInMessage';
import { localizeNumber, isValidNumber } from '../../../backbone/utils/number';

const defaultFilter = {
  category: 'all',
  type: 'all',
  shipsTo: 'any',
  searchTerm: '',
  sortBy: 'PRICE_ASC',
  freeShipping: false,
};

export default {
  components: {
    CategoryFilter,
    TypeFilter,
    ListingsGrid,
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
      countryList: getTranslatedCountries(),
      filter: { ...defaultFilter },
      listingsViewType: app.localSettings.get('listingsGridViewType'),

      inputTerm: '',
      storeListingsCol: {},
      listingsGridKey: 0,

      // Standard width grid has 3 columns, so best to leave this
      // as a multiple of 3.
      LISTINGS_PER_PAGE: 24,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.render();
  },
  unmounted() {
  },
  computed: {
    ob() {
      const isFetching = this.fetch && this.fetch.state() === 'pending';
      const fetchFailed = this.fetch && this.fetch.state() === 'rejected' && this.fetch.status !== 404;

      return {
        ...this.templateHelpers,
        ...this._model,
        isFetching,
        fetchFailed,
        fetchFailReason: (fetchFailed && this.fetch.responseJSON && this.fetch.responseJSON.reason) || '',
        filter: this.filter,
        countryList: this.countryList,
        shipsToSelected: this.filter.shipsTo || 'any',
        listingCount: this.collection.length,
      };
    },
    filteredCollection() {
      const filter = this.filter;
      const collection = this.collection;

      const models = collection.models.filter((md) => {
        let passesFilter = true;

        if (filter.freeShipping && !md.shipsFreeToMe) {
          passesFilter = false;
        }

        if (filter.category !== 'all' && md.get('categories').indexOf(filter.category) === -1) {
          passesFilter = false;
        }

        if (filter.type !== 'all' && md.get('contractType') !== filter.type) {
          passesFilter = false;
        }

        const searchTerm = filter.searchTerm;

        if (searchTerm && md.searchTitle.indexOf(searchTerm) === -1 && md.searchDescription.indexOf(searchTerm) === -1) {
          passesFilter = false;
        }

        if (filter.shipsTo !== 'any' && !md.shipsTo(filter.shipsTo)) {
          passesFilter = false;
        }

        return passesFilter;
      });

      let col = new Listings(models, { guid: this.model.id });
      this.setSortFunction(col);
      col.sort();

      // todo: exceptionally tall screens may fit an entire page
      // with room to spare. Which means no scrollbar, which means subsequent
      // pages will not load. Handle that case.
      this.storeListingsCol = new Listings(col.slice(0, this.LISTINGS_PER_PAGE), { guid: this.model.id });
      this.listingsGridKey += 1;

      return col;
    },
    

    fullListingCount() {
      const col = this.filteredCollection;

      const countPhrase = app.polyglot.t('userPage.store.countListings', { smart_count: col.length, display_count: localizeNumber(col.length) });

      return app.polyglot.t('userPage.store.countListingsFound', { countListings: `<span class="txB">${countPhrase}</span>` });
    },
    enableShipping() {
      return this.filter.type === 'PHYSICAL_GOOD' || this.filter.type === 'all';
    },
  },
  watch: {
    _collection() {
      this.$emit('listingsUpdate', this._collection);
    }
  },
  methods: {
    loadData(options = {}) {
      this.baseInit(options);

      if (!this.collection) {
        throw new Error('Please provide a collection.');
      }

      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.listenTo(this.collection, 'request', this.onRequest);
      this.listenTo(this.collection, 'update', this.onUpdateCollection);

      if (this.model.id === app.profile.id) {
        this.listenTo(listingEvents, 'saved', (md, opts) => {
          // For now, we only know if the listing model has
          // changed in some way since the last save. We don't
          // know what specifically changed. So, this message
          // will show if some listing attribute changed, even
          // though it may not be one represented in the store.
          if (opts.hasChanged()) {
            this.showDataChangedMessage();
          }
        });

        this.listenTo(listingEvents, 'destroy', () => this.showDataChangedMessage());
      }

      this.listenTo(app.settings, 'change:country', () => this.showShippingChangedMessage());
      this.listenTo(app.settings, 'change:localCurrency', () => this.showDataChangedMessage());
      this.listenTo(app.localSettings, 'change:bitcoinUnit', () => this.showDataChangedMessage());

      this.listenTo(app.settings.get('shippingAddresses'), 'update', (cl, opts) => {
        if (opts.changes.added.length || opts.changes.removed.length) {
          this.showShippingChangedMessage();
        }
      });

      // this block should be last
      this.fetch = this.fetchListings();
      this.onRequest(this.collection, this.fetch);
    },

    onUpdateCollection(cl, opts) {
      if (opts.changes.added) {
        opts.changes.added.forEach((md) => {
          md.searchDescription = $('<div />')
            .html(md.get('description') || '')
            .text()
            .toLocaleLowerCase();
          md.searchTitle = (md.get('title') || '').toLocaleLowerCase();
          const price = md.get('price');

          if (
            price &&
            isValidNumber(price.amount, {
              allowNumber: false,
              allowBigNumber: true,
              allowString: false,
            })
          ) {
            try {
              md.convertedPrice = convertCurrency(price.amount, price.currencyCode, app.settings.get('localCurrency'));
            } catch (e) {
              if (e instanceof NoExchangeRateDataError) {
                // If no exchange rate data is available, we'll just use the unconverted price
                md.convertedPrice = price.amount;
              } else {
                throw e;
              }
            }
          }
        });
      }
    },

    onKeyupSearchInput(term) {
      // make sure they're not still typing
      if (this.searchKeyUpTimer) {
        clearTimeout(this.searchKeyUpTimer);
      }

      this.searchKeyUpTimer = setTimeout(() => this.search(term), 150);
    },

    onRequest(cl, xhr) {
      // Ignore a request on the ListingShort model, which happens
      // if we delete it.
      if (!(cl instanceof Listings)) return;

      this.fetch = xhr;
      if (!this.retryPressed) this.render();

      const startTime = Date.now();

      xhr.always(() => {
        if (xhr.state() === 'rejected' && xhr.status !== 404) {
          // if fetch is triggered by retry button and
          // it immediately fails, it looks like nothing happend,
          // so, we'll make sure it takes a minimum time.
          const callTime = Date.now() - startTime;

          if (callTime < 250) {
            setTimeout(() => {
              this.retryPressed = false;
              this.render();
            }, 250 - callTime);
          } else {
            this.retryPressed = false;
            this.render();
          }
        } else {
          this.retryPressed = false;
          this.render();
        }
      });
    },

    onClickRetryFetch() {
      this.retryPressed = true;
      this.fetchListings();
      this.$btnRetry().addClass('processing');
    },

    onClickClearSearch() {
      // will reset filters / search text, but maintain sort
      this.filter = {
        ...defaultFilter,
        sortBy: this.filter.sortBy,
      };

      this.render();
    },

    onClickToggleListGridView() {
      const prevType = this.listingsViewType;
      this.listingsViewType = prevType === 'list' ? 'grid' : 'list';
    },

    onClickActivateStore() {
      launchSettingsModal({ initialTab: 'Store' });
    },
    showDataChangedMessage() {
      if (this.dataChangePopIn && !this.dataChangePopIn.isRemoved()) {
        this.dataChangePopIn.$el.velocity('callout.shake', { duration: 500 });
      } else {
        this.dataChangePopIn = this.createChild(PopInMessage, {
          messageText: buildRefreshAlertMessage(app.polyglot.t('userPage.store.listingDataChangedPopin')),
        });

        this.listenTo(this.dataChangePopIn, 'clickRefresh', () => this.fetchListings());

        this.listenTo(this.dataChangePopIn, 'clickDismiss', () => {
          this.dataChangePopIn.remove();
          this.dataChangePopIn = null;
        });

        this.$popInMessages().append(this.dataChangePopIn.render().el);
      }
    },

    showShippingChangedMessage() {
      if (this.shippingChangePopIn && !this.shippingChangePopIn.isRemoved()) {
        this.shippingChangePopIn.$el.velocity('callout.shake', { duration: 500 });
      } else {
        this.shippingChangePopIn = this.createChild(PopInMessage, {
          messageText: buildRefreshAlertMessage(app.polyglot.t('userPage.store.shippingDataChangedPopin')),
        });

        this.listenTo(this.shippingChangePopIn, 'clickRefresh', () => this.fetchListings());

        this.listenTo(this.shippingChangePopIn, 'clickDismiss', () => {
          this.shippingChangePopIn.remove();
          this.shippingChangePopIn = null;
        });

        this.$popInMessages().append(this.shippingChangePopIn.render().el);
      }
    },

    fetchListings() {
      return this.collection.fetch({ cache: false });
    },

    search(term) {
      const searchTerm = term.toLocaleLowerCase().trim();

      if (searchTerm === this.filter.searchTerm) return;

      this.filter.searchTerm = searchTerm;
    },

    /**
     * When a listing card is clicked, the listingShort view will manage showing the
     * listing detail modal. This method is used when this view initially loads and a
     * listing was part of the url. Since we don't want to wait until the entire
     * (unpaginated) store is fetched before showing the listing, we are expecting the
     * the listing model to be passed in as an arg (currently the router is fetching it).
     * The store will continue to load in the background.
     */
    showListing(listing) {
      if (!listing instanceof Listing) {
        throw new Error('Please provide a listing model.');
      }

      const onListingDetailClose = () => app.router.navigate(`${this.model.id}/store`);

      this.listingDetail = new ListingDetail({
        profile: this.model,
        model: listing,
      })
        .render()
        .open();

      this.listenTo(this.listingDetail, 'close', onListingDetailClose);
      this.listenTo(this.listingDetail, 'modal-will-remove', () => this.stopListening(null, null, onListingDetailClose));
    },
    $btnRetry() {
      return $('.js-retryFetch');
    },
    $popInMessages() {
      return $('.js-storePopInMessages');
    },

    /**
     * Based on the sortBy filter, will appropriatally set the
     * comparator value on the given collection.
     */
    setSortFunction(col) {
      if (!col) {
        throw new Error('Please provide a collection.');
      }

      if (this.filter.sortBy) {
        if (this.filter.sortBy === 'PRICE_ASC') {
          col.comparator = (a, b) => {
            if (a.convertedPrice > b.convertedPrice) {
              return 1;
            } else if (a.convertedPrice < b.convertedPrice) {
              return -1;
            }

            return 0;
          };
        } else if (this.filter.sortBy === 'PRICE_DESC') {
          col.comparator = (a, b) => {
            if (a.convertedPrice < b.convertedPrice) {
              return 1;
            } else if (a.convertedPrice > b.convertedPrice) {
              return -1;
            }

            return 0;
          };
        } else if (this.filter.sortBy === 'NAME_ASC') {
          col.comparator = (a, b) => {
            if (a.get('title').toLocaleLowerCase() > b.get('title').toLocaleLowerCase()) {
              return 1;
            } else if (a.get('title').toLocaleLowerCase() < b.get('title').toLocaleLowerCase()) {
              return -1;
            }

            return 0;
          };
        } else if (this.filter.sortBy === 'NAME_DESC') {
          col.comparator = (a, b) => {
            if (a.get('title').toLocaleLowerCase() < b.get('title').toLocaleLowerCase()) {
              return 1;
            } else if (a.get('title').toLocaleLowerCase() > b.get('title').toLocaleLowerCase()) {
              return -1;
            }

            return 0;
          };
        }
      }
    },

    _onStoreListingsScroll(e) {
      let currentLength = this.storeListingsCol.length;
      // if we've scrolled within a 150px of the bottom
      if (e.target.scrollTop + $(e.target).innerHeight() >= e.target.scrollHeight - 150) {
        this.storeListingsCol.add(this.filteredCollection.slice(currentLength, currentLength + this.LISTINGS_PER_PAGE));
      }
    },

    onStoreListingsScroll(e) {
      _.debounce(() => {
        this._onStoreListingsScroll(e);
      }, 100);
    },

    onCategoryChange(cat) {
      this.filter.category = cat;
    },

    onTypeChange(type) {
      this.filter.type = type;
    },

    render() {
      if (this.dataChangePopIn) this.dataChangePopIn.remove();
      if (this.shippingChangePopIn) this.shippingChangePopIn.remove();

      if (!this.rendered) {
        if (this.listing) {
          // if first render, show a listing if it was
          // passed in as a view option
          this.showListing(this.listing);
        }

        this.rendered = true;
      }

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
