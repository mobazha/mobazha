<template>
  <div class="row flexVBase gutterH">
    <div class="tx5 flexExpand">
      <div v-if="ob.results">
        <div v-if="ob.term" v-html="ob.polyT('search.resultsFound', {
          term: ob.parseEmojis(ob.term),
          smart_count: ob.number.localizeNumber(ob.results.total),
        })">
        </div>

        <div v-else>
          <b>
            {{ ob.polyT('search.resultsTotal', {
              smart_count: ob.number.localizeNumber(ob.results.total),
            }) }}
          </b>
        </div>
        <span class="toolTip" :data-tip="ob.polyT('search.resultsHelper')">
          <i class="ion-information-circled clrT2"></i>
        </span>
      </div>
    </div>
    <div v-if="ob.sortBy">
      <div class="tx5b">
        {{ ob.polyT('search.sortBy') }}
      </div>
      <div class="col4">
        <select id="sortBy" class="select2Small " @change="changeSortBy($event)">
          <option v-for="(val, key) in ob.sortBy" :key="key" :value="key" :selected="selected(val, key)">{{ val.label }}
          </option>
        </select>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import { selectEmojis } from '../../../backbone/utils';


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
    $('#sortBy').select2({
      minimumResultsForSearch: Infinity, // disables the search box
      templateResult: selectEmojis,
      templateSelection: selectEmojis,
    });
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        ...options,
        initialState: {
          ...options.initialState,
        },
      };

      this.baseInit(opts);
    },

    changeSortBy (event) {
      this.$emit('changeSortBy', { sortBy: event.target.value });
    },

    selected (val, key) {
      const ob = this.ob;

      let selected = false;
      if (ob.sortBySelected) {
        selected = key === ob.sortBySelected;
      } else {
        selected = val.default;
      }
      return selected;
    }

  }
}
</script>
<style lang="scss" scoped></style>
