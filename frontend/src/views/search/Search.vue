<template>
  <div class="search">
    <nav id="pageTabBar" :class="`noTabs barLg clrP clrBr ${ob.fetching ? 'noTips' : ''}`">
      <div class="pageTabs searchProviders flexRow gutterH">
        <div class="thumb discoverLogo flexNoShrink"></div>
        <div class="providersHeader flexNoShrink">
          <div class="flexVCent">
            <div>
              <div class="tx4 rowTn">Mobazha</div>
              <div class="tx6">{{ ob.polyT('search.title') }}</div>
            </div>
          </div>
        </div>

        <div class="categories flexRow gutterH">
          <a 
            v-for="category in categories" 
            :key="category.id"
            :class="`btn clrP clrBr navBtn ${isActiveCategory(category.id) ? 'active' : ''}`"
            @click="selectCategory(category.id)"
          >
            {{ category.name }}
          </a>
        </div>
        
        <div>
          <div class="flexVCent gutterHSm">
            <a class="btn barBtn flexNoShrink tx6 clrP clrBr clrSh2" href="#transactions/sales">{{
              ob.polyT('search.providers.transactions') }}</a>
            <a class="btn barBtn flexNoShrink tx6 clrP clrBr clrSh2" :href="`#${ob.peerID}`">{{
              ob.polyT('search.providers.myPage') }}</a>
          </div>
        </div>
      </div>
    </nav>

    <div v-if="!ob.fetching" class="pageContent">
      <template v-if="!ob.showDataError">
        <div class="flexColRows row">
          <div class="flexVBase gutterH">
            <h3 class="txUnl rowSm">{{ ob.name }}</h3>
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
            <button class="btn clrP clrBr searchBtn" @click="clickSearchBtn">
              {{ ob.polyT('search.searchBtn') }}
            </button>
          </div>
          <div class="js-suggestions">
            <Suggestions @clickSuggestion="onClickSuggestion" />
          </div>
          <hr class="clrBr" />
        </div>
        
        <!-- 搜索结果区域 -->
        <div class="flexRow gutterHLg" v-if="ob.tab === 'listings'" >
          <div v-if="ob.hasFilters" class="col3 filterWrapper js-filterWrapper">
            <Filters ref="filters" v-model:filters="ob.data.options" @filterChanged="onFilterChanged"/>
          </div>
          <div :class="`${ob.hasFilters ? 'col9' : 'col12'}`">
            <div class="flexCol">
              <div class="width100 js-sortByWrapper">
                <SortBy 
                  :options="sortByOptions"
                  @changeSortBy="changeSortBy"
                />
              </div>
              <div class="width100 js-resultsWrapper">
                <Results
                  :options="resultOptions"
                  @searchError="handleSearchError"
                  @loadingPage="scrollPageIntoView"
                  @resetSearch="resetSearch"
                />
              </div>
            </div>
          </div>
        </div>
      </template>
      <!-- 错误提示 -->
      <div v-else class="contentBox padLg flexColRows flexHCent clrP clrBr">
        <h2 class="rowLg">{{ ob.errTitle }}</h2>
        <p>{{ ob.errMsg }}</p>
      </div>
    </div>
    <!-- 加载中状态 -->
    <div v-else class="flexCent loadingSearch clrS">
      <SpinnerSVG className="spinnerLg" style="width: 80px; height: 80px" />
    </div>
  </div>
</template>

<script>
import _ from 'underscore';
import $ from 'jquery';
import app from '../../../backbone/app';
import { myGet } from '../../api/api';
import ResultsCol from '../../../backbone/collections/Results';
import { supportedWalletCurs } from '../../../backbone/data/walletCurrencies';
import { recordEvent } from '../../../backbone/utils/metrics';
import { scrollPageIntoView } from '../../../backbone/utils/dom';
import { createSearchURL } from '../../../backbone/utils/search';

import Suggestions from './Suggestions.vue'
import Filters from './Filters.vue'
import SortBy from './SortBy.vue'
import Results from './Results.vue'

export default {
  name: 'Search',
  
  components: {
    Suggestions,
    Filters, 
    SortBy,
    Results
  },

  data() {
    return {
      _state: {
        fetching: false,
        tab: 'listings',
        xhr: null,
        data: null,
        selectedCategory: 'all'
      },
      _search: {
        q: '*',
        p: 0,
        ps: 66,
        searchType: 'listings',
        provider: app.searchProviders.at(0),
        filters: {
          nsfw: String(app.settings.get('showNsfw')),
          acceptedCurrencies: supportedWalletCurs(),
          category: 'all'
        }
      },
      categories: [
        { id: 'all', name: this.templateHelpers.polyT('formats.ALL') },
        { id: 'physical_goods', name: this.templateHelpers.polyT('formats.PHYSICAL_GOOD') },
        { id: 'digital_goods', name: this.templateHelpers.polyT('formats.DIGITAL_GOOD') },
        { id: 'services', name: this.templateHelpers.polyT('formats.SERVICE') }
      ]
    }
  },

  computed: {
    ob() {
      const state = this._state;
      const data = state.data || {};
      const term = this._search.q === '*' ? '' : this._search.q;
      const hasFilters = data.options && !$.isEmptyObject(data);

      let errTitle = '';
      let errMsg = '';

      if (state.xhr) {
        errTitle = app.polyglot.t('search.errors.searchFailTitle', { 
          provider: this._search.provider.get('name') 
        });
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
        showDataError: $.isEmptyObject(data) && state.tab === 'listings',
        hasFilters,
        ...state,
        ...data,
      };
    },

    sortByOptions() {
      return {
        term: this.ob.term,
        results: this.ob.data?.results,
        sortBy: this.ob.data?.sortBy,
        sortBySelected: this._search.sortBy,
      }
    },

    resultOptions() {
      const data = this._state.data || {};
      this.resultsCol = new ResultsCol();
      this.resultsCol.add(this.resultsCol.parse(data));

      return {
        search: this._search,
        initCol: this.resultsCol,
        viewType: this.getViewType(data),
        setHistory: true
      }
    }
  },

  created() {
    this.fetchSearch(this._search);
  },

  methods: {
    getViewType(data) {
      if (this._search.filters.category === 'services') {
        return 'list';
      }
      
      if (data.options?.type?.options) {
        const cryptoOption = data.options.type.options.find(
          op => op.value === 'cryptocurrency' && op.checked
        );
        if (cryptoOption && data.options.type.options.filter(op => op.checked).length === 1) {
          return 'cryptoList';
        }
      }
      return 'grid';
    },

    scrollPageIntoView,

    async fetchSearch(opts = {}) {
      this.setState({ fetching: true, xhr: null });

      try {
        const data = await myGet(createSearchURL(opts));
        this.setState({
          fetching: false,
          data,
          tab: 'listings'
        });
      } catch (xhr) {
        if (xhr.statusText !== 'abort') {
          this.setState({
            fetching: false,
            data: {},
            xhr
          });
        }
      }
    },

    setState(newState) {
      this._state = {
        ...this._state,
        ...newState
      };
    },

    setSearch(search = {}, opts = {}) {
      const newSearch = { 
        ...this._search,
        ...search,
        filters: {
          ...this._search.filters,
          ...(search.filters || {})
        }
      };
      
      if (!_.isEqual(this._search, newSearch) || opts.force) {
        this._search = newSearch;
        scrollPageIntoView();
        this.fetchSearch(this._search);
      }
    },

    // Event handlers
    clickSearchBtn() {
      this.setSearch({ q: $('.js-searchInput').val(), p: 0 }, { force: true });
      recordEvent('Discover_ClickSearch');
      recordEvent('Discover_Search', { type: 'click' });
    },

    onKeyupSearchInput() {
      this.setSearch({ q: $('.js-searchInput').val(), p: 0 }, { force: true });
      recordEvent('Discover_EnterKeySearch');
      recordEvent('Discover_Search', { type: 'enterKey' });
    },

    changeSortBy(opts) {
      this.setSearch({ ...opts, p: 0 });
      recordEvent('Discover_ChangeSortBy');
    },

    onFilterChanged() {
      this.setSearch({ 
        filters: this.$refs.filters.retrieveFormData(), 
        p: 0 
      });
      recordEvent('Discover_ChangeFilter');
    },

    onClickSuggestion(opts) {
      this.setSearch({ 
        q: opts.suggestion, 
        p: 0, 
        filters: { type: 'all' } 
      });
      recordEvent('Discover_ClickSuggestion');
      recordEvent('Discover_Search', { type: 'suggestion' });
    },

    handleSearchError(xhr) {
      this.setState({
        fetching: false,
        data: {},
        xhr,
      });
    },

    resetSearch() {
      this.setSearch(this._search);
    },

    isActiveCategory(categoryId) {
      return this._search.filters.category === categoryId;
    },

    selectCategory(categoryId) {
      this.setSearch({ 
        filters: {
          ...this._search.filters,
          category: categoryId
        },
        p: 0 
      });
      recordEvent('Discover_ChangeCategory');
    }
  }
}
</script>

<style scoped>
.pageNav {
  padding: 10px 20px;
  border-bottom: 1px solid var(--border);
}

.navBtn {
  margin-right: 10px;
  transition: all 0.2s ease;
}

.navBtn:last-child {
  margin-right: 0;
}

.flex-grow {
  flex-grow: 1;
}

.categories {
  display: flex;
  align-items: center;
}

.categories .btn.active {
  background-color: var(--border);
  color: var(--primary);
  font-weight: 500;
  border-color: var(--primary);
}

.categories .btn:hover {
  background-color: var(--border);
  opacity: 0.8;
}
</style>
