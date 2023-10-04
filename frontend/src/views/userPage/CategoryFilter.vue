<template>
  <div class="clrP clrBr padMd clrT contentBox clrSh2 form veryCompact categoryOrTypeFilter">
    <!-- categoryFilter.html and typeFilter.html share quite a bit. Ensure that when one is updated,
     the other is also maintained. -->
    <div class="txB rowSm">{{ ob.polyT('userPage.store.categoryFilter.heading') }}</div>
    <div class="btnRadio">
      <input
        @change="onChangeCategory"
        type="radio"
        name="filterShippingCategory"
        value="all"
        id="filterShippingCategoryAll"
        data-var-type="boolean"
        :checked="ob.selected === 'all'"
      />
      <label for="filterShippingCategoryAll">{{ ob.polyT('userPage.store.categoryFilter.all') }}</label>
    </div>

    <template v-for="(cat, index) in ob.categories.slice(0, ob.maxInitiallyVisibleCats - 1)" :key="index">
      <div class="btnRadio">
        <input
          @change="onChangeCategory"
          type="radio"
          name="filterShippingCategory"
          :value="formatCategoryString(cat)"
          :id="`filterShippingCategory${flatCategoryString(cat)}`"
          :checked="ob.selected === cat"
        />
        <label :for="`filterShippingCategory${flatCategoryString(cat)}`">{{ cat }}</label>
      </div>
    </template>
    <!-- // adding 1 to the length to account for the All category we hard-code -->
    <template v-if="ob.categories.length + 1 > ob.maxInitiallyVisibleCats">
      <div :class="`js-moreCatsWrap moreCatsWrap ${ob.expanded ? 'expanded' : ''}`">
        <div class="moreCats">
          <template v-for="(cat, index) in ob.categories.slice(ob.maxInitiallyVisibleCats - 1)" :key="index">
            <div class="btnRadio">
              <input
                @change="onChangeCategory"
                type="radio"
                name="filterShippingCategory"
                :value="cat"
                :id="`filterShippingCategory${flatCategoryString(cat)}`"
                :checked="ob.selected === cat"
              />
              <label :for="`filterShippingCategory${flatCategoryString(cat)}`">{{ cat }}</label>
            </div>
          </template>
        </div>
        <a class="clrT tx6 txU showMore" @click="onClickShowMoreLess">{{
          ob.polyT('userPage.store.categoryFilter.showMore', ob.categories.length + 1 - ob.maxInitiallyVisibleCats)
        }}</a>
        <a class="clrT tx6 txU showLess" @click="onClickShowMoreLess">{{ ob.polyT('userPage.store.categoryFilter.showLess') }}</a>
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

    this.loadData(this.options);
  },
  mounted() {
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
    formatCategoryString(cat) {
      return cat.replace(/&/g, '&amp;');
    },
    flatCategoryString(cat) {
      // remove spaces
      return cat.replace(/\s/g, '-');
    },
    loadData(options = {}) {
      const opts = {
        ...options,
      };

      opts.initialState = {
        categories: [],
        selected: 'all',
        expanded: false,
        maxInitiallyVisibleCats: 6,
        ...(options.initialState || {}),
      };

      this.baseInit(opts);
    },
    onClickShowMoreLess() {
      this.setState({ expanded: !this.getState().expanded });
    },
    onChangeCategory(e) {
      this._state.selected = e.target.value;
      this.trigger('category-change', { value: $(e.target).val() });
    },
  },
};
</script>
<style lang="scss" scoped></style>
