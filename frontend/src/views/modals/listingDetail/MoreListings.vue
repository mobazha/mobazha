<template>
  <div class="moreListings">
    <div class="contentBox padLg clrP clrBr clrSh3">
      <template v-if="ob.vendor && ob.vendor.name">
        <h2 class="txUnb">{{ ob.polyT('listingDetail.moreBy', { name: ob.vendor.name }) }}</h2>
      </template>
      <div class="listingsGrid flex js-cardWrapper">
        <template v-for="listing in ob.listings" :key="listing.slug">
          <ListingCard
            :options="{
              listingBaseUrl: `${ob.vendor.peerID}/store/`,
              vendor: options.vendor,
              onStore: true,
            }"
            :bb="cardBB(listing)"
          />
        </template>
      </div>
    </div>

  </div>
</template>

<script>
import ListingShort from '../../../../backbone/models/listing/ListingShort';

export default {
  props: {
    options: {
      type: Object,
      default: {
        listings: [],
        vendor: {},
      },
    },
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this.options,
      };
    },
  },
  methods: {
    cardBB(listing) {
      const model = new ListingShort(listing);
      model.set('vendor', this.options.vendor);

      return function() {
        return {
          model,
        }
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
