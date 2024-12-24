<template>
  <div :class="`listingsGrid flex ${viewType === 'list' ? 'listingsGridListView' : ''}`" :key="viewType">
    <template v-for="model in collection" :key="model.cid">
      <ListingCard :options="getCardOptions(model)" :bb="function() {
          return {
            model,
          }
        }"
      />
    </template>
  </div>
</template>

<script>
import app from '../../../backbone/app';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    viewType: {
      type: String,
      default: app.localSettings.get('listingsGridViewType'),
    },
    bb: Function,
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () { },
  computed: {
  },
  watch: {
    viewType(type) {
      if (['list', 'grid'].indexOf(type) === -1) {
        throw new Error('The type provided is not one of the available types.');
      }

      // This just sets the flag. It's up to you to re-render to update the UI.
      if (app.localSettings.get('listingsGridViewType') !== type) {
        app.localSettings.save('listingsGridViewType', type);
      }
    },
  },
  methods: {
    getCardOptions(model) {
      let listingBaseUrl;

      // The listingBaseUrl can be directly provided as an option or we
      // will attempt to derive it from a passed in Profile model or
      // Vendor information in the listing short models.
      if (this.options.listingBaseUrl) {
        listingBaseUrl = this.options.listingBaseUrl;
      } else if (model.get('vendor')) {
        const base = model.get('vendor').handle ? `@${model.get('vendor').handle}` : model.get('vendor').peerID;
        listingBaseUrl = `${base}/store/`;
      } else if (this.storeOwnerProfile) {
        const base = this.storeOwnerProfile.get('handle') ? `@${this.storeOwnerProfile.get('handle')}` : this.storeOwnerProfile.id;
        listingBaseUrl = `${base}/store/`;
      }

      return {
        listingBaseUrl,
        viewType: this.viewType,
        vendor: {
          peerID: this.storeOwnerProfile.id,
          name: this.storeOwnerProfile.get('name'),
          handle: this.storeOwnerProfile.get('handle'),
          avatarHashes: this.storeOwnerProfile.get('avatarHashes').toJSON(),
        },

        // Flag so the listing card knows it's on a store. This is useful to
        // the listing detail modal and will be passed into there.
        onStore: true,
      };
    }
  },
};
</script>
<style lang="scss" scoped></style>
