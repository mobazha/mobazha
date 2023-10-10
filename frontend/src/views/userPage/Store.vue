<template>
  <div class="userPageStore">
    <div class="popInMessageHolder js-popInMessages"></div>
    <div class="userPageSearchBar flex gutterHSm" :disabled="ob.isFetching || ob.fetchFailed || !ob.listingCount">
      <div class="flexExpand">
        <div class="searchWrapper">
          <input type="text" class="ctrl searchInput clrP clrBr clrSh2" @keyup="onKeyupSearchInput" :placeholder="ob.polyT('userPage.searchStore')" />
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
          <div class="js-shippingFilterContainer clrP clrBr padMd clrT clrSh2 contentBox">
            <div class="txB rowSm">{{ ob.polyT('userPage.store.shippingFilter.heading') }}</div>
            <div class="flexVCent rowSm">
              <label class="margRSm" for="shipsToSelect">{{ ob.polyT('userPage.store.shippingFilter.shipsTo') }}:</label>
              <select class="tx6 select2Small js-shipsToSelect" @change="onShipsToSelectChange" style="width: 133px" id="shipsToSelect">
                <option value="any" :selected="ob.shipsToSelected === 'any'">{{ ob.polyT('userPage.store.shipsToFilterAny') }}</option>
                <option v-for="(country, j) in ob.countryList" :key="j" :value="country.dataName" :selected="country.dataName === ob.shipsToSelected">
                  {{ country.name }}
                </option>
              </select>
              <div class="select2Small js-shipsToSelectDropdownContainer"></div>
            </div>
            <div class="flexVCent rowSm">
              <input type="checkbox" id="filterFreeShipping" class="margRSm" @change="onFilterFreeShippingChange" :checked="ob.filter.freeShipping" />
              <label for="filterFreeShipping"></label
              ><!-- label for the replacement checkbox -->
              <label class="clrE1 clrTOnEmph phraseBox" for="filterFreeShipping">{{ ob.polyT('userPage.store.freeShippingBanner') }}</label>
            </div>
          </div>
          <div class="js-catFilterContainer"></div>
          <div class="js-typeFilterContainer"></div>
        </div>
        <div class="col storeListings">
          <template v-if="ob.listingCount">
            <div class="row clrT tx5 flexVBot listingsHeaderRow">
              <span class="listingsCount js-listingCount"></span>
              <div>
                <div class="tx6 flexVCent">
                  <label class="clrT2 marginLAuto margRSm">{{ ob.polyT('userPage.store.sortBy') }}</label>
                  <select class="tx6 select2Small js-sortBySelect" @change="onChangeSortBy" style="width: 150px">
                    <option value="PRICE_ASC" :selected="ob.filter.sortBy === 'PRICE_ASC'">{{ ob.polyT('userPage.store.sortOpts.priceAsc') }}</option>
                    <option value="PRICE_DESC" :selected="ob.filter.sortBy === 'PRICE_DESC'">{{ ob.polyT('userPage.store.sortOpts.priceDesc') }}</option>
                    <option value="NAME_ASC" :selected="ob.filter.sortBy === 'NAME_ASC'">{{ ob.polyT('userPage.store.sortOpts.nameAsc') }}</option>
                    <option value="NAME_DESC" :selected="ob.filter.sortBy === 'NAME_DESC'">{{ ob.polyT('userPage.store.sortOpts.nameDesc') }}</option>
                    <!-- <option value="RATING">Rating</option> -->
                  </select>
                  <div class="select2Small js-sortBySelectDropdownContainer"></div>
                </div>
              </div>
            </div>
            <div class="contentBox row pad clrP clrBr js-inactiveWarning" v-show="!ob.vendor">
              <span class="tx5"
                ><template v-html="ob.parseEmojis('🔒')"/> {{ ob.polyT('userPage.store.inactive') }}
                <button class="btnTxtOnly txU txUnb clrT2" @click="onClickActivateStore">${ob.polyT('userPage.store.inactiveLink')}</button>
              </span>
            </div>
            <div class="js-listingsContainer"></div>
            <div class="txCtr padGi clrP clrSh2 clrBr tx4 contentBox js-noResults hide">
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
import { getContentFrame } from '../../../backbone/utils/selectors';
import { convertCurrency, NoExchangeRateDataError } from '../../../backbone/utils/currency';
import { launchSettingsModal } from '../../../backbone/utils/modalManager';
import Listing from '../../../backbone/models/listing/Listing';
import Listings from '../../../backbone/collections/Listings';
import { events as listingEvents } from '../../../backbone/models/listing';
import ListingDetail from '../../../backbone/views/modals/listingDetail/Listing';
import ListingsGrid, { LISTINGS_PER_PAGE } from '../../../backbone/views/userPage/ListingsGrid';
import CategoryFilter from './CategoryFilter';
import TypeFilter from './TypeFilter';
import PopInMessage, { buildRefreshAlertMessage } from '../components/PopInMessage';
import { localizeNumber, isValidNumber } from '../../../backbone/utils/number';

export default {
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
      defaultFilter: {
        category: 'all',
        type: 'all',
        shipsTo: 'any',
        searchTerm: '',
        sortBy: 'PRICE_ASC',
        freeShipping: false,
      },
      filter: { ...this.defaultFilter },
      listingsViewType: app.localSettings.get('listingsGridViewType'),
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
    getContentFrame().off('scroll', this.storeListingsScrollHandler);
  },
  computed: {
    ob() {
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
        // if the user changes their vendor setting, toggle the warning
        this.listenTo(app.profile, 'change:vendor', () => this.toggleInactiveWarning());
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
      if (options.initialFetch) {
        this.fetch = options.initialFetch;
        this.onRequest(this.collection, this.fetch);
      }
    },

    onFilterFreeShippingChange(e) {
      this.filter.freeShipping = $(e.target).is(':checked');
      this.renderListings(this.filteredCollection());
    },

    toggleInactiveWarning() {
      this.$inactiveWarning().toggleClass('hide', app.settings.get('vendor'));
    },

    onShipsToSelectChange(e) {
      this.filter.shipsTo = e.target.value;
      this.renderListings(this.filteredCollection());
    },

    onChangeSortBy(e) {
      this.filter.sortBy = $(e.target).val();
      this.renderListings();
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

    onKeyupSearchInput(e) {
      // make sure they're not still typing
      if (this.searchKeyUpTimer) {
        clearTimeout(this.searchKeyUpTimer);
      }

      this.searchKeyUpTimer = setTimeout(() => this.search($(e.target).val()), 150);
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
        ...this.defaultFilter,
        sortBy: this.filter.sortBy,
      };

      this.render();
    },

    onClickToggleListGridView() {
      const prevType = this.listingsViewType;
      let type = prevType === 'list' ? 'grid' : 'list';
      this.listingsViewType = type;
      if (prevType) {
        if (prevType !== type) {
          this.$el.toggleClass('listView');
          if (this.storeListings) {
            this.renderListings(this.fullRenderedCollection);
          }
        }
      } else if (type === 'list') {
        this.$el.addClass('listView');
      }
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

    fetchListings(options = {}) {
      Store.fetchListings(this.collection, options);
    },

    search(term) {
      const searchTerm = term.toLocaleLowerCase().trim();

      if (searchTerm === this.filter.searchTerm) return;

      this.filter.searchTerm = searchTerm;
      this.renderListings(this.filteredCollection());
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
    $listingsContainer() {
      return $('.js-listingsContainer');
    },
    $shippingFilterContainer() {
      return $('.js-shippingFilterContainer');
    },
    $catFilterContainer() {
      return $('.js-catFilterContainer');
    },
    $typeFilterContainer() {
      return $('.js-typeFilterContainer');
    },
    $listingCount() {
      return $('.js-listingCount');
    },
    $noResults() {
      return $('.js-noResults') || null;
    },
    $popInMessages() {
      return $('.js-popInMessages');
    },
    $inactiveWarning() {
      return $('.js-inactiveWarning');
    },
    filteredCollection(filter = this.filter, collection = this.collection) {
      const models = collection.models.filter((md) => {
        let passesFilter = true;

        if (this.filter.freeShipping && !md.shipsFreeToMe) {
          passesFilter = false;
        }

        if (this.filter.category !== 'all' && md.get('categories').indexOf(this.filter.category) === -1) {
          passesFilter = false;
        }

        if (this.filter.type !== 'all' && md.get('contractType') !== this.filter.type) {
          passesFilter = false;
        }

        const searchTerm = this.filter.searchTerm;

        if (searchTerm && md.searchTitle.indexOf(searchTerm) === -1 && md.searchDescription.indexOf(searchTerm) === -1) {
          passesFilter = false;
        }

        if (this.filter.shipsTo !== 'any' && !md.shipsTo(this.filter.shipsTo)) {
          passesFilter = false;
        }

        return passesFilter;
      });

      return new Listings(models, { guid: this.model.id });
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

    storeListingsScroll(paginatedCol, e) {
      // Make sure we're in the DOM (i.e. the store tab is active).
      if (!this.el.parentElement) return;

      // if we've scrolled within a 150px of the bottom
      if (e.target.scrollTop + $(e.target).innerHeight() >= e.target.scrollHeight - 150) {
        paginatedCol.add(this.fullRenderedCollection.slice(this.storeListings.listingCount, this.storeListings.listingCount + LISTINGS_PER_PAGE));
      }
    },

    renderListings(col = this.fullRenderedCollection) {
      if (!col) {
        throw new Error('Please provide a collection.');
      }

      // This collection will be loaded in batches as the
      // user scrolls.
      this.fullRenderedCollection = col;
      this.setSortFunction(col);
      col.sort();

      this.$listingsContainer().empty();

      const countPhrase = app.polyglot.t('userPage.store.countListings', { smart_count: col.length, display_count: localizeNumber(col.length) });

      const fullListingCount = app.polyglot.t('userPage.store.countListingsFound', { countListings: `<span class="txB">${countPhrase}</span>` });
      this.$listingCount().html(fullListingCount);

      if (col.length) {
        // todo: exceptionally tall screens may fit an entire page
        // with room to spare. Which means no scrollbar, which means subsequent
        // pages will not load. Handle that case.
        const storeListingsCol = new Listings(col.slice(0, LISTINGS_PER_PAGE), { guid: this.model.id });

        if (this.storeListings) this.storeListings.remove();

        this.storeListings = new ListingsGrid({
          collection: storeListingsCol,
          storeOwnerProfile: this.model,
          viewType: this.listingsViewType,
        });

        getContentFrame().on('scroll', this.storeListingsScrollHandler);
        const scrollHandler = (e) => this.storeListingsScroll.call(this, storeListingsCol, e);
        this.storeListingsScrollHandler = _.debounce(scrollHandler, 100);
        getContentFrame().on('scroll', this.storeListingsScrollHandler);

        this.$noResults().addClass('hide');
        this.$listingsContainer().append(this.storeListings.render().el);
      } else {
        this.$noResults().removeClass('hide');
      }
    },

    renderCategories(cats = this.collection.categories) {
      if (!this.categoryFilter) {
        this.categoryFilter = new CategoryFilter({
          initialState: {
            categories: cats,
            selected: this.filter.category,
          },
        });

        this.categoryFilter.render();

        this.listenTo(this.categoryFilter, 'category-change', (e) => {
          this.filter.category = e.value;
          this.renderListings(this.filteredCollection());
        });
      } else {
        if (cats.indexOf(this.filter.category) === -1) {
          this.filter.category = 'all';
        }

        this.categoryFilter.setState({
          categories: cats,
          selected: this.filter.category,
        });
      }

      if (!this.$catFilterContainer()[0].contains(this.categoryFilter.el)) {
        this.categoryFilter.delegateEvents();
        this.$catFilterContainer().empty().append(this.categoryFilter.el);
      }
    },

    renderTypes(types = this.collection.types) {
      if (!this.typeFilter) {
        this.typeFilter = new TypeFilter({
          initialState: {
            types,
            selected: this.filter.type,
          },
        });

        this.typeFilter.render();

        this.listenTo(this.typeFilter, 'type-change', (e) => {
          this.filter.type = e.value;

          if (this.filter.type !== 'PHYSICAL_GOOD' && this.filter.type !== 'all') {
            this.$shippingFilterContainer().addClass('disabled');
          } else {
            this.$shippingFilterContainer().removeClass('disabled');
          }

          this.renderListings(this.filteredCollection());
        });
      } else {
        if (types.indexOf(this.filter.type) === -1) {
          this.filter.type = 'all';
        }

        this.typeFilter.setState({
          types,
          selected: this.filter.type,
        });
      }

      if (!this.$typeFilterContainer()[0].contains(this.typeFilter.el)) {
        this.typeFilter.delegateEvents();
        this.$typeFilterContainer().empty().append(this.typeFilter.el);
      }
    },

    render() {
      if (this.dataChangePopIn) this.dataChangePopIn.remove();
      if (this.shippingChangePopIn) this.shippingChangePopIn.remove();

      const isFetching = this.fetch && this.fetch.state() === 'pending';
      const fetchFailed = this.fetch && this.fetch.state() === 'rejected' && this.fetch.status !== 404;

      this.$sortBy = this.$('.js-sortBySelect');
      this.$shipsToSelect = this.$('.js-shipsToSelect');

      this.$sortBy.select2({
        minimumResultsForSearch: -1,
        dropdownParent: this.$('.js-sortBySelectDropdownContainer'),
      });

      this.$shipsToSelect.select2({
        dropdownParent: this.$('.js-shipsToSelectDropdownContainer'),
      });

      if (!this.rendered) {
        if (this.options.listing) {
          // if first render, show a listing if it was
          // passed in as a view option
          this.showListing(this.options.listing);
        }

        this.rendered = true;
      }

      if (!isFetching && !fetchFailed) {
        this.renderCategories(this.collection.categories);
        this.renderTypes(this.collection.types);

        if (this.collection.length) {
          this.renderListings(this.filteredCollection());
        }
      }

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>

<!-- 
Store.fetchListings = (cl, options = {}) =>
  cl.fetch({ cache: false, ...options }); -->
