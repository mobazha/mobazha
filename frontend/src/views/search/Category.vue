<template>
  <div class="searchCategory searchResults flexColRow">
    <a class="clrT " @click="clickSeeAll">
      <h2>{{ ob.title }}</h2>
    </a>
    <template v-if="ob.viewType === 'cryptoList'">
      <div class="flexVCent txB clrBr clrP gutterH cryptoListViewHeader">
        <div class="tradeFromCol">{{ ob.polyT('search.cryptoListViewHeader.colTradeFrom') }}</div>
        <div class="tradeArrowCol"></div>
        <div class="tradeToCol">{{ ob.polyT('search.cryptoListViewHeader.colTradeTo') }}</div>
        <div class="vendorCol">{{ ob.polyT('search.cryptoListViewHeader.colVendor') }}</div>
        <div class="priceCol"
          v-html='ob.polyT("search.cryptoListViewHeader.colPrice", { subText: `<span class="subText clrT2">${ob.polyT("search.cryptoListViewHeader.colPriceSubText")}</span>` })' />
        <!-- /* This is being commented out until inventory is functional.
        <div class="inventoryCol flexExpand">
          {{ ob.polyT('search.cryptoListViewHeader.colInventory') }} <span class="toolTip txCtr" :data-tip="ob.polyT('search.cryptoListViewHeader.tipInventory')"><i class="ion-information-circled clrT2"></i></span>
        </div> -->
      </div>
    </template>
    <div :class="`listingsGrid ${ob.viewTypeClass} flex js-resultsGrid`">
      <template v-for="model in catCol" :key="model.cid">
        <ListingCard
          :options="cardViewOptions(model)"
          :bb="function() {
            return {
              model,
            };
          }"
        />
      </template>
    </div>
    <div class="flexCent rowLg">
      <button class="btn clrP clrT clrBr clrSh1 " @click="clickSeeAll">{{ ob.polyT('search.categories.seeAllBtn') }}</button>
    </div>
    <hr class="clrBr row categoryRow">
    <template v-if="ob.loading">
      <div class="flexCent loadingSearch clrS">
        <SpinnerSVG className="spinnerLg" />
      </div>
    </template>

  </div>
</template>

<script>
import { capitalize } from '../../../backbone/utils/string';
import { recordEvent } from '../../../backbone/utils/metrics';
import { createSearchURL } from '../../../backbone/utils/search';
import ResultsCol from '../../../backbone/collections/Results';
import ProviderMd from '../../../backbone/models/search/SearchProvider';


export default {
  props: {
    options: {
      type: Object,
      default: {
        search: {},
        viewType: '',
      },
    },
  },
  data () {
    return {
      cryptoTitle: '',

      _state: {
        loading: false,
      }
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
        viewTypeClass: this.viewType === 'grid' ?
          '' : `listingsGrid${capitalize(this.viewType)}View`,
        viewType: this.viewType,
        title: this.cryptoTitle || this._search.q,
        ...this._state,
      };
    }
  },
  unmounted() {
    this.removeCardViews();
    if (this.categoryFetch) this.categoryFetch.abort();
  },
  methods: {
    loadData (options = {}) {
      if (!options.search) throw new Error('Please provide a search object.');
      if (!options.search.provider || !(options.search.provider instanceof ProviderMd)) {
        throw new Error('Please provide a provider model.');
      }
      const { initialState = {}, ...restOptions } = options;
      const opts = {
        viewType: 'grid',
        ...restOptions,
        initialState: {
          loading: false,
          ...initialState,
        },
      };
      this.baseInit(opts);

      this._search = {
        p: 0,
        ps: 8,
        ...options.search,
      };

      if (this._search.filters.type === 'cryptocurrency' && this._search.q === '*') {
        this.cryptoTitle = 'Trade';
      }

      this.cardViews = [];
      this.catCol = new ResultsCol();
      this.loadCategory();
    },

    clickSeeAll () {
      recordEvent('Discover_SeeAllCategory', { category: this.cryptoTitle || this._search.q });
      this.$emit('seeAllCategory', { q: this._search.q, filters: this._search.filters });
    },

    cardViewOptions (model) {
      const vendor = model.get('vendor') || {};
      const base = vendor.handle ? `@${vendor.handle}` : vendor.peerID;
      return {
        listingBaseUrl: `${base}/store/`,
        reportsUrl: this._search.provider.reportsUrl || '',
        searchUrl: this._search.provider.listingsUrl,
        vendor,
        onStore: false,
        viewType: this.viewType,
      };
    },

    loadCategory (options) {
      this.removeCardViews();
      this.setState({ loading: true });

      const opts = {
        ...this._search,
        ...options,
      };

      if (this.categoryFetch) this.categoryFetch.abort();
      if (this.catCol.length) this.catCol.reset();

      this.categoryFetch = this.catCol.fetch({
        url: createSearchURL(opts),
      })
        .done(() => {
          this.$emit('fetchComplete');
        })
        .fail((xhr) => {
          if (xhr.statusText !== 'abort') this.$emit('searchError', xhr);
        })
        .always(() => {
          this.setState({ loading: false });
        });
    },

    removeCardViews () {
      this.cardViews.forEach((vw) => vw.remove());
      this.cardViews = [];
    },
  }
}
</script>
<style lang="scss" scoped></style>
