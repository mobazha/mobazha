<template>
  <div :class="`listingsGrid flex ${viewType === 'list' ? 'listingsGridListView' : ''}`">
    <template v-for="model in collection">
      <ListingCard :options="cardOptions(model)" :bb="function() {
        return {
          model,
        }
      }" />
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
    this.loadData(this.options);
  },
  mounted () { },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.options,
      };
    },
    listingCount() {
      return this.collection.length;
    },
  },
  watch: {
    viewType(type) {
      if (['list', 'grid'].indexOf(type) === '-1') {
        throw new Error('The type provided is not one of the available types.');
      }

      // This just sets the flag. It's up to you to re-render to update the UI.
      app.localSettings.save('listingsGridViewType', type);
    },
  },
  methods: {
    loadData (options = {}) {
      // If this grid is for a Store, pass in a storeOwnerProfile option
      // with the Profile model of the store owner.

      this.baseInit(options);

      if (!this.collection) {
        throw new Error('Please provide a collection.');
      }

      if (!this.options.listingBaseUrl && !this.storeOwnerProfile) {
        let allHaveVendor = true;

        try {
          this.collection.forEach(md => {
            if (!md.get('vendor')) throw new Error();
          });
        } catch (e) {
          allHaveVendor = false;
        }

        if (!allHaveVendor) {
          throw new Error('I am unable to determine a listingBaseUrl for one or more of the ' +
            'provided listings. Please either pass in a listingBaseUrl option or it can be ' +
            'derived if you provid a storeOwnerProfile option or every model has an embedded ' +
            ' Vendor object.');
        }
      }
    },

    cardOptions(model) {
      let listingBaseUrl;

      // The listingBaseUrl can be directly provided as an option or we
      // will attempt to derive it from a passed in Profile model or
      // Vendor information in the listing short models.
      if (this.options.listingBaseUrl) {
        listingBaseUrl = this.options.listingBaseUrl;
      } else if (model.get('vendor')) {
        const base = model.get('vendor').handle ?
          `@${model.get('vendor').handle}` : model.get('vendor').peerID;
        listingBaseUrl = `${base}/store/`;
      } else if (this.storeOwnerProfile) {
        const base = this.storeOwnerProfile.get('handle') ?
          `@${this.storeOwnerProfile.get('handle')}` :
          this.storeOwnerProfile.id;
        listingBaseUrl = `${base}/store/`;
      }

      const options = {
        listingBaseUrl,
        viewType: this.viewType,
      };

      if (this.storeOwnerProfile) {
        options.profile = this.storeOwnerProfile;
        // Flag so the listing card knows it's on a store. This is useful to
        // the listing detail modal and will be passed into there.
        options.onStore = true;
      }

      return options;
    },
  },
};
</script>
<style lang="scss" scoped></style>
  