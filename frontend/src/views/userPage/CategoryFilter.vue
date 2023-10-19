<template>
  <div class="clrP clrBr padMd clrT contentBox clrSh2 form veryCompact categoryOrTypeFilter">
    <!-- categoryFilter.html and typeFilter.html share quite a bit. Ensure that when one is updated,
     the other is also maintained. -->
    <div class="txB rowSm">{{ ob.polyT('userPage.store.categoryFilter.heading') }}</div>
    <div class="btnRadio">
      <input
        @change="onChangeCategory('all')"
        type="radio"
        name="filterShippingCategory"
        value="all"
        id="filterShippingCategoryAll"
        data-var-type="boolean"
        :checked="selected === 'all'"
      />
      <label for="filterShippingCategoryAll">{{ ob.polyT('userPage.store.categoryFilter.all') }}</label>
    </div>

    <template v-for="(cat, index) in categories.slice(0, maxInitiallyVisibleCats - 1)" :key="index">
      <div class="btnRadio">
        <input
          @change="onChangeCategory(cat)"
          type="radio"
          name="filterShippingCategory"
          :value="formatCategoryString(cat)"
          :id="`filterShippingCategory${flatCategoryString(cat)}`"
          :checked="selected === cat"
        />
        <label :for="`filterShippingCategory${flatCategoryString(cat)}`" v-html="cat"></label>
      </div>
    </template>
    <!-- // adding 1 to the length to account for the All category we hard-code -->
    <template v-if="categories.length + 1 > maxInitiallyVisibleCats">
      <div :class="`js-moreCatsWrap moreCatsWrap ${expanded ? 'expanded' : ''}`">
        <div class="moreCats">
          <template v-for="(cat, index) in categories.slice(maxInitiallyVisibleCats - 1)" :key="index">
            <div class="btnRadio">
              <input
                @change="onChangeCategory(cat)"
                type="radio"
                name="filterShippingCategory"
                :value="cat"
                :id="`filterShippingCategory${flatCategoryString(cat)}`"
                :checked="selected === cat"
              />
              <label :for="`filterShippingCategory${flatCategoryString(cat)}`" v-html="cat"></label>
            </div>
          </template>
        </div>
        <a class="clrT tx6 txU showMore" @click="onClickShowMoreLess">{{
          ob.polyT('userPage.store.categoryFilter.showMore', categories.length + 1 - maxInitiallyVisibleCats)
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
    categories: {
      type: Object,
      default: [],
    },
    selected: {
      type: String,
      default: 'all',
    },
    maxInitiallyVisibleCats: {
      type: Number,
      default: 6,
    }
  },
  data() {
    return {
      expanded: false,
    };
  },
  created() {
  },
  mounted() {
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
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
    onClickShowMoreLess() {
      this.expanded = !this.expanded;
    },
    onChangeCategory(val) {
      this.$emit('category-change', val);
    },
  },
};
</script>
<style lang="scss" scoped></style>
