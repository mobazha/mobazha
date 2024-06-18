<template>
  <div class="searchResults flexColRow gutterV">
    <template v-if="ob.viewType === 'cryptoList'">
      <div class="flexVCent txB clrBr clrP gutterH cryptoListViewHeader">
        <div class="tradeFromCol">{{ ob.polyT('search.cryptoListViewHeader.colTradeFrom') }}</div>
        <div class="tradeArrowCol"></div>
        <div class="tradeToCol">{{ ob.polyT('search.cryptoListViewHeader.colTradeTo') }}</div>
        <div class="vendorCol">{{ ob.polyT('search.cryptoListViewHeader.colVendor') }}</div>
        <div class="priceCol" v-html='ob.polyT("search.cryptoListViewHeader.colPrice",
          { subText: `<span class="subText clrT2">${ob.polyT("search.cryptoListViewHeader.colPriceSubText")}</span>` })'>

        </div>
        <!-- /* This is being commented out until inventory is functional.
        <div class="inventoryCol flexExpand">{{ ob.polyT('search.cryptoListViewHeader.colInventory') }} <span class="toolTip txCtr" :data-tip="ob.polyT('search.cryptoListViewHeader.tipInventory')"><i class="ion-information-circled clrT2"></i></span></div>
        */ -->
      </div>
    </template>
    <div class="noResultsMessage contentBox clrP clrBr clrSh3">
      <div class="padHg">
        <p class="txCtr">{{ ob.polyT('search.noResults') }}</p>
      </div>
      <div class="flexRow flexBtnWrapper">
        <a class="btnFlx flexExpand clrP clrBr " @click="clickResetBtn">Reset Search</a>
      </div>
    </div>
    <div ref="resultsGrid" :class="`listingsGrid ${ob.viewTypeClass} flex js-resultsGrid`">
      <template v-for="model in pageCols[_search.p]" :key="`${model.get('hash')}_${model.get('slug')}`">
        <ListingCard v-if="isListingCardModel(model)"
          :options="cardViewOptions(model)"
          :bb="function() {
            return {
              model,
            };
          }"
        />
        <UserCard v-else
          :bb="function() {
            return {
              model,
            };
          }"
        />
      </template>
    </div>
    <div class="pageControls js-pageControlsContainer">
      <PageControlsTextStyle :options="{
        currentPage,
        morePages,
      }"
      @clickNext="clickPageNext"
      @clickPrev="clickPagePrev"/>
    </div>
    <hr class="clrBr">
    <template v-if="loading">
      <div class="flexCent loadingSearch clrS">
        <SpinnerSVG className="spinnerLg" />
      </div>
    </template>

  </div>
</template>

<script>
import app from '../../../backbone/app';
import { capitalize } from '../../../backbone/utils/string';
import { recordEvent } from '../../../backbone/utils/metrics';
import { createSearchURL } from '../../../backbone/utils/search';
import ResultsCol from '../../../backbone/collections/Results';
import ListingCardModel from '../../../backbone/models/listing/ListingShort';
import ProviderMd from '../../../backbone/models/search/SearchProvider';


export default {
  props: {
    options: {
      type: Object,
      default: {
        search: {},
        initCol: {},
        viewType: '',
        setHistory: false,
      },
    },
  },
  data () {
    return {
      loading: false,
      pageCols: [],
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        viewTypeClass: this.viewType === 'grid' ? '' : `listingsGrid${capitalize(this.viewType)}View`,
        viewType: this.viewType,
      };
    },

    currentPage(){
      return Number(this._search.p) + 1;
    },
    pageCol() {
      return this.pageCols[this._search.p];
    },
    morePages() {
      return this.currentPage < Math.ceil(this.pageCol.total / this._search.ps);
    }
  },
  unmounted() {
    this.removeCardViews();
    if (this.newPageFetch) this.newPageFetch.abort();
  },
  methods: {
    loadData (options = {}) {
      if (!options.search) throw new Error('Please provide a search object.');
      if (!options.search.provider || !(options.search.provider instanceof ProviderMd)) {
        throw new Error('Please provide a provider model.');
      }

      this.baseInit(options);
      this._setHistory = options.setHistory;
      this._search = {
        p: 0,
        ps: 66,
        ...options.search,
      };

      this.cardViews = [];
      this.pageCols = {};
      // if an initial collection was passed in, add it
      if (options.initCol) this.pageCols[this._search.p] = (options.initCol);
      this.loadPage();
    },

    clickResetBtn () {
      this.$emit('resetSearch');
    },

    cardViewOptions(model) {
      // models can be listings or nodes, even though nodes aren't being used yet
      const vendor = model.get('vendor') || {};
      const base = vendor.handle ? `@${vendor.handle}` : vendor.peerID;
      return {
        listingBaseUrl: `${base}/store/`,
        reportsUrl: this._search.provider.reportsUrl || '',
        searchUrl: this._search.provider[this._search.urlType],
        vendor,
        onStore: false,
        viewType: this.viewType,
      };
    },

    isListingCardModel(model) {
      return model instanceof ListingCardModel;
    },

    loadPage (options) {
      this.removeCardViews();
      this.$emit('loadingPage');
      this.loading = true;

      const opts = {
        ...this._search,
        ...options,
      };

      const newUrl = createSearchURL(opts);

      // if page exists, reuse it
      if (this.pageCols[opts.p]) {
        if (this._setHistory) {
          app.router.navigate(`search/listings?providerQ=${encodeURIComponent(newUrl)}`);
        }
        this.loading = false;
      } else {
        const newPageCol = new ResultsCol();
        this.pageCols[opts.p] = newPageCol;

        if (this.newPageFetch) this.newPageFetch.abort();

        this.newPageFetch = newPageCol.fetch({
          url: newUrl,
        })
          .done(() => {
            if (this._setHistory) {
              app.router.navigate(`search/listings?providerQ=${encodeURIComponent(newUrl)}`);
            }
          })
          .fail((xhr) => {
            if (xhr.statusText !== 'abort') this.trigger('searchError', xhr);
          })
          .always(() => {
            this.loading = false;
          });
      }
    },

    clickPagePrev () {
      recordEvent('Discover_PrevPage', { fromPage: this._search.p });
      this._search.p--;
      this._setHistory = true;
      this.loadPage();
    },

    clickPageNext () {
      recordEvent('Discover_NextPage', { fromPage: this._search.p });
      this._search.p++;
      this._setHistory = true;
      this.loadPage();
    },

    removeCardViews () {
      this.cardViews.forEach((vw) => vw.remove());
      this.cardViews = [];
    },
  }
}
</script>
<style lang="scss" scoped></style>
