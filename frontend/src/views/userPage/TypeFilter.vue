<template>
  <div class="clrP clrBr padMd clrT contentBox clrSh2 form veryCompact categoryOrTypeFilter">
    <!-- categoryFilter.html and typeFilter.html share quite a bit. Ensure that when one is updated,
     the other is also maintained. -->
    <div class="txB rowSm">{{ ob.polyT('userPage.store.typeFilter.heading') }}</div>
    <div class="btnRadio">
      <input type="radio" name="filterListingType" @change="onChangeType('all')" id="filterListingTypeAll" data-var-type="boolean" :checked="selected === 'all'" />
      <label for="filterListingTypeAll">{{ ob.polyT('userPage.store.typeFilter.all') }}</label>
    </div>

    <template v-for="(type, index) in types.slice(0, maxInitiallyVisibleTypes - 1)" :key="index">
      <div class="btnRadio">
        <input type="radio" name="filterListingType" @change="onChangeType(type)" :id="`filterListingType${flatType(type)}`" :checked="selected === type" />
        <label :for="`filterListingType${flatType(type)}`">{{ ob.polyT(`formats.${type}`) }}</label>
      </div>
    </template>
    <!-- // adding 1 to the length to account for the All type we hard-code %> -->
    <template v-if="types.length + 1 > maxInitiallyVisibleTypes">
      <div :class="`js-moreTypesWrap moreTypesWrap ${expanded ? 'expanded' : ''}`">
        <div class="moreTypes">
          <template v-for="(type, index) in types.slice(maxInitiallyVisibleTypes - 1)" :key="index">
            <div class="btnRadio">
              <input type="radio" name="filterListingType" @change="onChangeType(type)" :id="`filterListingType${flatType(type)}`" :checked="selected === type" />
              <label :for="`filterListingType${flatType(type)}`">{{ ob.polyT(`formats.${type}`) }}</label>
            </div>
          </template>
        </div>
        <a class="clrT tx6 txU showMore" @click="onClickShowMoreLess">{{
          ob.polyT('userPage.store.typeFilter.showMore', types.length + 1 - maxInitiallyVisibleTypes)
        }}</a>
        <a class="clrT tx6 txU showLess" @click="onClickShowMoreLess">{{ ob.polyT('userPage.store.typeFilter.showLess') }}</a>
      </div>
    </template>
  </div>
</template>

<script>
/* categoryFilter.js and typeFilter.js share quite a bit. Ensure that when one is updated,
the other is also maintained.
*/

export default {
  props: {
    types: {
      type: Object,
      default: [],
    },
    selected: {
      type: String,
      default: 'all',
    },
    maxInitiallyVisibleTypes: {
      type: Number,
      default: 6,
    },
  },
  data() {
    return {
      expanded: false,
    };
  },
  created() {
  },
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
      };
    },
  },
  methods: {
    flatType(type) {
      return type.replace(/\s/g, '-');
    },

    onClickShowMoreLess() {
      this.expanded = !this.expanded;
    },

    onChangeType(val) {
      this.$emit('type-change', val);
    },
  },
};
</script>
<style lang="scss" scoped></style>
