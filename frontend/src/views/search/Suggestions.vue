<template>
  <div class="suggestions flex gutterH row tx5 noOverflow">
    <div v-if="ob.suggestions && ob.suggestions.length">
      <span class="clrT2">{{ ob.polyT('search.suggestions') }}</span>
      <div v-for="(suggestion, j) in ob.suggestions" :key="j">
        <a class="clrT " @click="onClickSuggestion" :data-suggestion="suggestion">{{ suggestion }}</a>
      </div>
    </div>

  </div>
</template>

<script>
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

    this.loadData(this.options);
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
    loadData (options = {}) {
      const opts = {
        initialState: {
          suggestions: [
            'Books',
            'Art',
            'Clothing',
            'Bitcoin',
            'Crypto',
            'Handmade',
            'Health',
            'Toys',
            'Electronics',
            'Games',
            'Music',
          ],
          ...options.initialState || {},
        },
        ...options,
      };

      this.baseInit(opts);
    },

    events () {
      return {
      };
    },

    onClickSuggestion (e) {
      const suggestion = $(e.target).data('suggestion');
      this.$emit('clickSuggestion', { suggestion });
    },
  }
}
</script>
<style lang="scss" scoped></style>
