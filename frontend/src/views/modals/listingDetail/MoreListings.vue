<template>
  <div class="moreListings">
    <div class="contentBox padLg clrP clrBr clrSh3">
      <template v-if="ob.vendor && ob.vendor.name">
        <h2 class="txUnb">{{ ob.polyT('listingDetail.moreBy', { name: ob.vendor.name }) }}</h2>
      </template>
      <div class="listingsGrid flex js-cardWrapper">
        <template v-for="listing in ob.listings">
          <ListingCard
            :options="cardViewOptions(listing).options"
            :bb="function() {
              return {
                model:cardViewOptions(listing).model,
              };
            }"
            @listingDetailOpened="onListingDetailOpened"
          />
        </template>
      </div>
    </div>

  </div>
</template>

<script>
import ListingShort from '../../../../backbone/models/listing/ListingShort';
import ListingCard from '../../components/ListingCard';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        listings: [],
        vendor: {},
      },
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit({
        ...options,
        initialState: {
          listings: [],
          vendor: {},
          ...options.initialState,
        },
      });
    },

    cardViewOptions (listing) {
      const vendor = this.getState().vendor;
      const model = new ListingShort(listing);
      model.set('vendor', vendor);
      return {
        model,
        options: {
          listingBaseUrl: `${vendor.peerID}/store/`,
          vendor,
          onStore: true,
        }
      };
    },

    onListingDetailOpened() {
      this.$emit('listingDetailOpened');
    },
  }
}
</script>
<style lang="scss" scoped></style>
