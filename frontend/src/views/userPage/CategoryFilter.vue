<template>
  <div class="clrP clrBr padMd clrT contentBox clrSh2 form veryCompact categoryOrTypeFilter">
    <!-- categoryFilter.html and typeFilter.html share quite a bit. Ensure that when one is updated,
     the other is also maintained. -->
    <div class="txB rowSm">{{ ob.polyT('userPage.store.categoryFilter.heading') }}</div>
    <div class="btnRadio">
      <input type="radio" name="filterShippingCategory" value="all" id="filterShippingCategoryAll" data-var-type="boolean"
        :checked="ob.selected === 'all'">
      <label for="filterShippingCategoryAll">{{ ob.polyT('userPage.store.categoryFilter.all') }}</label>
    </div>

    <div v-for="(cat, index) in ob.categories.slice(0, ob.maxInitiallyVisibleCats - 1)" :key="index">
      <div class="btnRadio">
        <input type="radio" name="filterShippingCategory" :value="formatCategoryString(cat)"
          :id="`filterShippingCategory${flatCategoryString(cat)}`" :checked="ob.selected === cat">
        <label :for="`filterShippingCategory${flatCategoryString(cat)}`">{{ cat }}</label>
      </div>
    </div>
    <!-- // adding 1 to the length to account for the All category we hard-code -->
    <div v-if="(ob.categories.length + 1) > ob.maxInitiallyVisibleCats">
      <div :class="`js-moreCatsWrap moreCatsWrap ${ob.expanded ? 'expanded' : ''}`">
        <div class="moreCats">
          <div v-for="(cat, index) in ob.categories.slice(ob.maxInitiallyVisibleCats - 1)" :key="index">
            <div class="btnRadio">
              <input type="radio" name="filterShippingCategory" :value="cat"
                :id="`filterShippingCategory${flatCategoryString(cat)}`" :checked="ob.selected === cat">
              <label :for="`filterShippingCategory${flatCategoryString(cat)}`">{{ cat }}</label>
            </div>
          </div>
        </div>
        <a class="clrT tx6 txU showMore" @click="onClickShowMoreLess">{{
          ob.polyT('userPage.store.categoryFilter.showMore', (ob.categories.length + 1) - ob.maxInitiallyVisibleCats)
        }}</a>
        <a class="clrT tx6 txU showLess" @click="onClickShowMoreLess">{{
          ob.polyT('userPage.store.categoryFilter.showLess') }}</a>
      </div>
    </div>

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
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    }
  },
  methods: {
    formatCategoryString (cat) {
      return cat.replace(/&/g, '&amp;');
    },
    flatCategoryString (cat) {
      // remove spaces
      return cat.replace(/\s/g, '-');
    },
    loadData (options = {}) {
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

      this.setState(opts.initialState || {});
      this.options = opts;
    },

    events () {
      return {
        'change input[type="radio"]': 'onChangeCategory',
      };
    },

    onClickShowMoreLess () {
      this.setState({ expanded: !this.getState().expanded });
    },

    onChangeCategory (e) {
      this._state.selected = e.target.value;
      this.trigger('category-change', { value: $(e.target).val() });
    },
  }
}
</script>
<style lang="scss" scoped></style>
