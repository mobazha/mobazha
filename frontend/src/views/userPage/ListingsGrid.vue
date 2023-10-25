<template>
  <div :class="`listingsGrid flex ${viewType === 'list' ? 'listingsGridListView' : ''}`" :key="viewType">
    <template v-for="model in collection" :key="model.id">
      <ListingCard :options="{
          viewType,
          profile: storeOwnerProfile,
          // Flag so the listing card knows it's on a store. This is useful to
          // the listing detail modal and will be passed into there.
          onStore: true,
        }" :bb="function() {
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
  },
};
</script>
<style lang="scss" scoped></style>
  