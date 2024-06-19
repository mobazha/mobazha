<template>
  <div class="search">
    <nav id="pageTabBar" :class="`noTabs barLg clrP clrBr ${ob.fetching ? 'noTips' : ''}`">
      <div class="pageTabs js-searchProviders">
        <Providers
          :options="{
            searchType: _search.searchType,
            currentID: _search.provider.id,
            showSelectDefault: !currentDefaultProvider,
          }"
          @activateProvider="activateProvider" />
      </div>
    </nav>

    <div v-if="!ob.fetching" class="pageContent">
      <template v-if="!ob.showDataError">
        <div class="flexColRows row">
          <div class="flexVBase gutterH">
            <h3 class="txUnl rowSm">{{ ob.name }}</h3>
            <template v-if="ob.isExistingProvider">
              <button v-if="!ob.providerLocked" class="btnTxtOnly txb txU txUnb" @click="clickDeleteProvider">
                {{ ob.polyT('search.deleteProviderBtn') }}
              </button>
              <button v-if="ob.showMakeDefault" class="btnTxtOnly txb txU txUnb" @click="clickMakeDefaultProvider">
                {{ ob.polyT('search.makeDefaultBtn') }}
                <span class="toolTip" :data-tip="ob.polyT('search.makeDefaultBtnHelper')">
                  <i class="ion-help-circled"></i>
                </span>
              </button>
            </template>
            <button v-else class="btnTxtOnly txb txU txUnb" @click="clickAddQueryProvider">{{ ob.polyT('search.addQueryProviderBtn') }}</button>
          </div>
          <div class="searchBar row clrP clrBr clrSh2">
            <div v-if="ob.logo" :class="`searchLogo js-searchLogo ${!ob.logo ? 'loadError' : ''}`">
              <img :src="ob.logo" />
            </div>
            <input
              :class="`clrP clrBr searchInput js-searchInput ${ob.logo ? 'withLogo' : ''}`"
              type="text"
              :placeholder="ob.polyT('search.searchPlaceholder')"
              :value="ob.term"
              @keyup.enter="onKeyupSearchInput"
            />
            <button class="btn clrP clrBr searchBtn" @click="clickSearchBtn">{{ ob.polyT('search.searchBtn') }}</button>
          </div>
          <div class="js-suggestions">
            <Suggestions @clickSuggestion="onClickSuggestion" />
          </div>
          <hr class="clrBr" />
        </div>
        <div class="js-categoryWrapper" v-if="ob.tab === 'home'">
          <template v-for="search in _categorySearches">
            <Category
              :options="{
                search,
                viewType: search.filters.type === 'cryptocurrency' ? 'cryptoList' : 'grid',
              }"
              @seeAllCategory="setSearch"
              />
          </template>
        </div>
        <div class="flexRow gutterHLg" v-if="ob.tab === 'listings'" >
          <div v-if="ob.hasFilters" class="col3 filterWrapper js-filterWrapper">
            <Filters ref="filters" v-model:filters="ob.data.options" @filterChanged="onFilterChanged"/>
          </div>
          <div :class="`${ob.hasFilters ? 'col9' : 'col12'}`">
            <div class="flexCol">
              <div class="width100 js-sortByWrapper">
                <SortBy :options="{
                  term: ob.term,
                  results: ob.data.results,
                  sortBy: ob.data.sortBy,
                  sortBySelected: _search.sortBy,
                }"
                @changeSortBy="changeSortBy"/>
              </div>
              <div class="width100 js-resultsWrapper">
                <Results
                  :options="resultOptions"
                  @searchError="(xhr) => {
                    setState({
                      fetching: false,
                      data: {},
                      xhr,
                    });
                  }"
                  @loadingPage="scrollPageIntoView"
                  @resetSearch="() => this.setSearch(this._defaultSearch)"
                  />
              </div>
            </div>
          </div>
        </div>
      </template>
      <div v-else class="contentBox padLg flexColRows flexHCent clrP clrBr">
        <h2 class="rowLg">{{ ob.errTitle }}</h2>
        <p>{{ ob.errMsg }}</p>
        <div v-if="!ob.providerLocked" class="flexHCent">
          <button class="btn clrP clrBr" @click="clickDeleteProvider">{{ ob.polyT('search.deleteProviderBtn') }}</button>
        </div>
      </div>
    </div>
    <div v-else class="flexCent loadingSearch clrS">
      <SpinnerSVG className="spinnerLg" style="width: 80px; height: 80px" />
    </div>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import $ from 'jquery';
import is from 'is_js';
import app from '../../../backbone/app';
import { openSimpleMessage } from '../../../backbone/views/modals/SimpleMessage';
import ResultsCol from '../../../backbone/collections/Results';
import ProviderMd from '../../../backbone/models/search/SearchProvider';
import { supportedWalletCurs } from '../../../backbone/data/walletCurrencies';
import defaultSearchProviders from '../../../backbone/data/defaultSearchProviders';
import { recordEvent } from '../../../backbone/utils/metrics';
import { curConnOnTor } from '../../../backbone/utils/serverConnect';
import { scrollPageIntoView } from '../../../backbone/utils/dom';
import {
  searchTypes,
  createSearchURL,
} from '../../../backbone/utils/search';

import Suggestions from './Suggestions.vue'
import Providers from './Providers.vue'
import Category from './Category.vue'
import Filters from './Filters.vue'
import SortBy from './SortBy.vue'
import Results from './Results.vue'


export default {
  components: {
    Suggestions,
    Providers,
    Category,
    Filters,
    SortBy,
    Results,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        fetching: false,
        tab: 'listings',
        xhr: null,
      },

      _search: {},
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  unmounted() {
    this.removeFetches();
    this.categoryViews.forEach((cat) => cat.remove());
  },
  computed: {
    ob () {
      const state = this._state;
      const data = state.data || {};
      const term = this._search.q === '*' ? '' : this._search.q;
      const hasFilters = data.options && !$.isEmptyObject(data);

      let errTitle;
      let errMsg;

      if (state.xhr) {
        const provider = this._search.provider.get('name') || this.currentBaseUrl;
        errTitle = app.polyglot.t('search.errors.searchFailTitle', { provider });
        const failReason = state.xhr.responseJSON ? state.xhr.responseJSON.reason : '';
        errMsg = failReason
          ? app.polyglot.t('search.errors.searchFailReason', { error: failReason })
          : app.polyglot.t('search.errors.searchFailData');
      }

      return {
        ...this.templateHelpers,
        term,
        errTitle,
        errMsg,
        providerLocked: this.providerIsADefault(this._search.provider.id),
        isExistingProvider: this.isExistingProvider(this._search.provider),
        showMakeDefault: this._search.provider !== this.currentDefaultProvider,
        showDataError: $.isEmptyObject(data) && state.tab === 'listings',
        hasFilters,
        ...state,
        ...data,
      };
    },
    currentDefaultProvider () {
      return app.searchProviders.defaultProvider;
    },
    currentBaseUrl () {
      return this._search.provider[`${this._search.searchType}Url`];
    },
    /**
     * This will create a results view from the provided search data.
     * @param {object} data - JSON results from a search endpoint.
     * @param {object} search - A valid search object.
     * @param {boolean} setHistory - Whether the results should save the query to history.
     */
    resultOptions () {
      const data = this._state.data || {};

      // Use the initial set of results data to create the results view.
      this.resultsCol = new ResultsCol();
      this.resultsCol.add(this.resultsCol.parse(data));

      let viewType = 'grid';

      if (data.options && data.options.type
        && data.options.type.options
        && data.options.type.options.length) {
        if (data.options.type.options.find((op) => op.value === 'cryptocurrency' && op.checked)
          && data.options.type.options.filter((op) => op.checked).length === 1) {
          viewType = 'cryptoList';
        }
      }

      return {
        search: this._search,
        initCol: this.resultsCol,
        viewType,
        setHistory: this._setHistory,
      }
    }
  },
  methods: {
    scrollPageIntoView,

    loadData (options = {}) {
      options.query = this.$route.query;

      const opts = {
        initialState: {
          fetching: false,
          tab: 'listings',
          xhr: null,
          ...options.initialState,
        },
        ...options,
      };

      this.baseInit(opts);
      const queryKeys = ['q', 'p', 'ps', 'sortBy'];

      // Allow router to pass in a search type for future use with vendor searches.
      const searchType = searchTypes.includes(opts.initialState.tab)
        ? opts.initialState.tab : 'listings';

      this._defaultSearch = {
        q: '*',
        p: 0,
        ps: 66,
        searchType,
        filters: {
          nsfw: String(app.settings.get('showNsfw')),
          acceptedCurrencies: supportedWalletCurs(),
        },
      };

      this._search = {
        ...this._defaultSearch,
        ..._.pick(opts, [...queryKeys, 'filters']),
      };

      // If there is only one provider and it isn't the default, just set it to be such.
      if (!this.currentDefaultProvider && app.searchProviders.length === 1) {
        this.setCurrentDefaultProvider(app.searchProviders.at(0));
      }
      this._search.provider = this.currentDefaultProvider || app.searchProviders.at(0);

      this._categoryTerms = [
        'Art',
        'Music',
        'Toys',
        'Crypto',
        'Books',
        'Health',
        'Games',
        'Handmade',
        'Clothing',
        'Electronics',
        'Bitcoin',
      ];

      this._categorySearch = {
        ...this._search,
        provider: app.searchProviders.at(0),
        ps: 8,
      };

      this._cryptoSearch = {
        ...this._categorySearch,
        ps: 5,
        filters: {
          type: 'cryptocurrency',
        },
      };

      this._categorySearches = [this._cryptoSearch];
      this._categoryTerms.forEach((cat) => {
        this._categorySearches.push({ ...this._categorySearch, q: cat });
      });

      this.categoryViews = [];
      this.searchFetches = [];
      this._setHistory = false; // The router has already set the history.

      // If a query was passed in from the router, extract the data from it.
      if (options.query) {
        recordEvent('Discover_SearchFromAddressBar');
        recordEvent('Discover_Search', { type: 'addressBar' });

        const queryParams = (new URL(`${this.currentBaseUrl}?${options.query}`)).searchParams;

        // If the query had a providerQ parameter, use that as the provider URL instead.
        if (queryParams.get('providerQ')) {
          const subURL = new URL(queryParams.get('providerQ'));
          queryParams.delete('providerQ');
          // The first parameter after the ? will be part of the providerQ, transfer it over.
          for (const param of subURL.searchParams.entries()) {
            queryParams.append(param[0], param[1]);
          }
          const base = `${subURL.origin}${subURL.pathname}`;
          /*
           If the query provider model doesn't already exist, create a new provider model for it.
           One quirk to note: if a tor url is passed in while the user is in clear mode, and an
           existing provider has that tor url, that provider will be activated but will use its
           clear url if it has one. The opposite is also true.
           */
          const matchedProvider = is.url(base) ? app.searchProviders.getProviderByURL(base) : '';
          if (!matchedProvider) {
            this._search.provider = new ProviderMd();
            /*
             We don't actually know what type of search the url is for, we'll assume for example a
             user in tor mode is only pasting in a tor url. If there is a mismatch, the correct
             values will be saved after the endpoint returns them.
             */
            const searchAttribute = `${curConnOnTor() ? 'tor' : ''}${this._search.searchType}`;
            this._search.provider.set(searchAttribute, base);
            if (!this._search.provider.isValid()) {
              openSimpleMessage(app.polyglot.t('search.errors.invalidUrl'));
              this._search.provider = app.searchProviders.at(0);
              recordEvent('Discover_InvalidQueryProvider', { url: base });
            }
          } else {
            this._search.provider = matchedProvider;
          }
        }

        const params = {};

        for (const key of queryParams.keys()) {
          // checkbox params are represented by the same key multiple times. Convert them into a
          // single key with an array of values
          const val = queryParams.getAll(key);
          params[key] = val.length === 1 ? val[0] : val;
        }

        // set the params in the search object
        const filters = { ...this._search.filters, ..._.omit(params, [...queryKeys]) };

        this.setSearch({ ..._.pick(params, ...queryKeys), filters }, { force: true });
      } else if (this._search.provider.id === defaultSearchProviders[0].id) {
        this.buildCategories();
      } else {
        this.setSearch({}, { force: true });
      }
    },

    isExistingProvider (md) {
      if (!md || !(md instanceof ProviderMd)) {
        throw new Error('Please provide a search provider model.');
      }
      return !!app.searchProviders.getProviderByURL(md[`${this._search.searchType}Url`]);
    },

    getCurrentDefaultProvider () {
      return app.searchProviders.defaultProvider;
    },

    setCurrentDefaultProvider (md) {
      if (!md || !(md instanceof ProviderMd)) {
        throw new Error('Please provide a search provider model.');
      }

      app.searchProviders[`default${curConnOnTor() ? 'Tor' : ''}Provider`] = md;
    },

    providerIsADefault (id) {
      return !!_.findWhere(defaultSearchProviders, { id });
    },

    /** Updates the search object. If updated, triggers a search fetch.
     *
     * @param {object} search - The new state.
     * @param {boolean} opts.force - Should search be fired even if nothing changed?
     */
    setSearch (search = {}, opts = {}) {
      const previousSearch = { ...this._search };
      const newSearch = { ...this._search, ...search, };

      if (!_.isEqual(previousSearch, newSearch) || opts.force) {
        this._search = newSearch;
        scrollPageIntoView();
        this.fetchSearch(this._search);
      }
    },

    /**
     * Creates an object for updating search providers with new data returned from a query.
     * @param {object} data - Provider object from a search query.
     * @returns {{data: *, urlTypes: Array}}
     */
    buildProviderUpdate (data) {
      const update = {};
      const urlTypes = [];

      if (data.name && is.string(data.name)) update.name = data.name;
      if (data.logo && is.url(data.logo)) update.logo = data.logo;
      if (data.links) {
        if (is.url(data.links.vendors)) {
          update.vendors = data.links.vendors;
          urlTypes.push('vendors');
        }
        if (is.url(data.links.listings)) {
          update.listings = data.links.listings;
          urlTypes.push('listings');
        }
        if (is.url(data.links.reports)) {
          update.reports = data.links.reports;
          urlTypes.push('reports');
        }
        if (data.links.tor) {
          if (is.url(data.links.tor.listings)) {
            update.torListings = data.links.tor.listings;
            urlTypes.push('torlistings');
          }
          if (is.url(data.links.tor.vendors)) {
            update.torVendors = data.links.tor.vendors;
            urlTypes.push('torVendors');
          }
          if (is.url(data.links.tor.reports)) {
            update.torReports = data.links.tor.reports;
            urlTypes.push('torReports');
          }
        }
      }

      return {
        update,
        urlTypes,
      };
    },

    fetchSearch (opts = {}) {
      this.removeFetches();

      this.setState({
        tab: 'listings',
        fetching: true,
        xhr: null,
      });

      const searchFetch = $.get({
        url: createSearchURL(opts),
        dataType: 'json',
      })
        .done((data, status, xhr) => {
          // make sure minimal data is present. If it isn't, it's probably an invalid endpoint.
          if (data.name && data.links) {
            const dataUpdate = this.buildProviderUpdate(data);

            // for browser mode, don't update search provider
            if (import.meta.env.VITE_APP) {
              // update the defaults but do not save them
              if (!this.providerIsADefault(this._search.provider.id)) {
                this._search.provider.save(dataUpdate.update, { urlTypes: dataUpdate.urlTypes });
              } else {
                this._search.provider.set(dataUpdate.update, { urlTypes: dataUpdate.urlTypes });
              }
            }

            this.setState({
              fetching: false,
              data,
            });
            // After either the first search or the first category load completes, set the history.
            this._setHistory = true;
          } else {
            this.setState({
              fetching: false,
              data: {},
              xhr,
            });
          }
        })
        .fail((xhr) => {
          if (xhr.statusText !== 'abort') {
            this.setState({
              fetching: false,
              data: {},
              xhr,
            });
          }
        });

      this.searchFetches.push(searchFetch);
    },

    /**
     * This will activate a provider. If no default is set, the activated provider will be set as the
     * the default. If the user is currently in Tor mode, the default Tor provider will be set.
     * @param {object} md - the search provider model
     */
    activateProvider (md) {
      if (!md || !(md instanceof ProviderMd)) {
        throw new Error('Please provide a search provider model.');
      }
      if (app.searchProviders.indexOf(md) === -1) {
        throw new Error('The provider must be in the collection.');
      }

      if (!this.currentDefaultProvider) this.makeDefaultProvider(md);

      if (md.id === defaultSearchProviders[0].id) {
        this.buildCategories();
      } else {
        this.setSearch({ provider: md, p: 0 });
      }
    },

    deleteProvider (md = this._search.provider) {
      // Default providers shouldn't show an option to trigger this.
      if (!this.providerIsADefault(md.id)) {
        md.destroy();
        if (app.searchProviders.length) this.activateProvider(app.searchProviders.at(0));
      }
    },

    clickDeleteProvider () {
      recordEvent('Discover_DeleteProvider', {
        provider: this._search.provider.get('name') || 'unknown',
        url: this.currentBaseUrl,
      });
      this.deleteProvider();
    },

    makeDefaultProvider (md) {
      if (!md || !(md instanceof ProviderMd)) {
        throw new Error('Please provide a search provider model.');
      }
      if (app.searchProviders.indexOf(md) === -1) {
        throw new Error('The provider to be made the default must be in the collection.');
      }

      this.setCurrentDefaultProvider(md);
    },

    clickMakeDefaultProvider () {
      recordEvent('Discover_MakeDefaultProvider', {
        provider: this._search.provider.get('name') || 'unknown',
        url: this.currentBaseUrl,
      });
      this.makeDefaultProvider(this._search.provider);
    },

    addQueryProvider () {
      if (!this.isExistingProvider(this._search.provider)) {
        app.searchProviders.add(this._search.provider);
      }
    },

    clickAddQueryProvider () {
      this.addQueryProvider();
    },

    /**
     * This will add the categories one by one in a loop. If the category views already exist, they
     * will be reused to prevent new calls to the search endpoint.
     */
    buildCategories () {
      if (this.categoryViews.length === this._categorySearches.length) {
        app.router.navigate('search');
        // After either the first search or the first category load completes, set the history.
        this._setHistory = true;
        this._search = { ...this._defaultSearch, provider: app.searchProviders.at(0) };
        scrollPageIntoView();
        const data = { name: defaultSearchProviders[0].name, logo: defaultSearchProviders[0].logo };
        // The state may not be changed here, so always fire a render.
        this.setState({ tab: 'home', data }, { renderOnChange: false });
        return;
      }
    },

    clickSearchBtn () {
      this.setSearch({ q: $('.js-searchInput').val(), p: 0 }, { force: true });
      recordEvent('Discover_ClickSearch');
      recordEvent('Discover_Search', { type: 'click' });
    },

    onKeyupSearchInput () {
      this.setSearch({ q: $('.js-searchInput').val(), p: 0 }, { force: true });
      recordEvent('Discover_EnterKeySearch');
      recordEvent('Discover_Search', { type: 'enterKey' });
    },

    changeSortBy (opts) {
      this.setSearch({ ...opts, p: 0 });
      recordEvent('Discover_ChangeSortBy');
    },

    onFilterChanged () {
      this.setSearch({ filters: this.$refs.filters.retrieveFormData(), p: 0 });
      recordEvent('Discover_ChangeFilter');
    },

    onClickSuggestion (opts) {
      this.setSearch({ q: opts.suggestion, p: 0, filters: { type: 'all' } });
      recordEvent('Discover_ClickSuggestion');
      recordEvent('Discover_Search', { type: 'suggestion' });
    },

    removeFetches () {
      this.searchFetches.forEach((fetch) => fetch.abort());
    },
  }
}
</script>
<style lang="scss" scoped></style>
