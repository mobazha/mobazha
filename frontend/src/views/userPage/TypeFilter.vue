<template>
  <div class="clrP clrBr padMd clrT contentBox clrSh2 form veryCompact categoryOrTypeFilter">
    <!-- categoryFilter.html and typeFilter.html share quite a bit. Ensure that when one is updated,
     the other is also maintained. -->
    <div class="txB rowSm">{{ ob.polyT('userPage.store.typeFilter.heading') }}</div>
    <div class="btnRadio">
      <input type="radio" name="filterListingType" value="all" id="filterListingTypeAll" data-var-type="boolean" :checked="ob.selected === 'all'" />
      <label for="filterListingTypeAll">{{ ob.polyT('userPage.store.typeFilter.all') }}</label>
    </div>

    <template v-for="(type, index) in ob.types.slice(0, ob.maxInitiallyVisibleTypes - 1)" :key="index">
      <div class="btnRadio">
        <input type="radio" name="filterListingType" :value="type" :id="`filterListingType${flatType(type)}`" :checked="ob.selected === type" />
        <label :for="`filterListingType${flatType(type)}`">{{ ob.polyT(`formats.${type}`) }}</label>
      </div>
    </template>
    <!-- // adding 1 to the length to account for the All type we hard-code %> -->
    <template v-if="ob.types.length + 1 > ob.maxInitiallyVisibleTypes">
      <div :class="`js-moreTypesWrap moreTypesWrap ${ob.expanded ? 'expanded' : ''}`">
        <div class="moreTypes">
          <template v-for="(type, index) in ob.types.slice(ob.maxInitiallyVisibleTypes - 1)" :key="index">
            <div class="btnRadio">
              <input type="radio" name="filterListingType" :value="type" :id="`filterListingType${flatType(type)}`" :checked="ob.selected === type" />
              <label :for="`filterListingType${flatType(type)}`">${ob.polyT(`formats.${type}`)}</label>
            </div>
          </template>
        </div>
        <a class="clrT tx6 txU showMore" @click="onClickShowMoreLess">{{
          ob.polyT('userPage.store.typeFilter.showMore', ob.types.length + 1 - ob.maxInitiallyVisibleTypes)
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

import $ from 'jquery';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {};
  },
  created() {
    this.initEventChain();

    this.loadData(this.$props.options);
  },
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        ...options,
      };

      opts.initialState = {
        types: [],
        selected: 'all',
        expanded: false,
        maxInitiallyVisibleTypes: 6,
        ...(options.initialState || {}),
      };

      this.setState(opts.initialState || {});
      this.options = opts;
    },
    flatType(type) {
      return type.replace(/\s/g, '-');
    },

    events() {
      return {
        'change input[type="radio"]': 'onChangeType',
      };
    },

    onClickShowMoreLess() {
      this.setState({ expanded: !this.getState().expanded });
    },

    onChangeType(e) {
      this._state.selected = e.target.value;
      this.$emit('type-change', { value: $(e.target).val() });
    },
  },
};
</script>
<style lang="scss" scoped></style>
