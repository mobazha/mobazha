<template>
  <div class="row flexVBase gutterH">
    <div class="tx5 flexExpand">
      <template v-if="options.results">
        <div v-if="options.term" v-html="ob.polyT('search.resultsFound', {
          term: ob.parseEmojis(options.term),
          smart_count: ob.number.localizeNumber(options.results.total),
        })">
        </div>

        <template v-else>
          <b>
            {{ ob.polyT('search.resultsTotal', {
              smart_count: ob.number.localizeNumber(options.results.total),
            }) }}
          </b>
        </template>
        <span class="toolTip" :data-tip="ob.polyT('search.resultsHelper')">
          <i class="ion-information-circled clrT2"></i>
        </span>
      </template>
    </div>
    <template v-if="options.sortBy">
      <div class="tx5b">
        {{ ob.polyT('search.sortBy') }}
      </div>
      <div class="col4">
        <select id="sortBy" class="select2Small " @change="changeSortBy($event)">
          <option v-for="(val, key) in options.sortBy" :key="key" :value="key" :selected="selected(val, key)">{{ val.label }}
          </option>
        </select>
      </div>
    </template>

  </div>
</template>

<script>
import $ from 'jquery';
import { selectEmojis } from '../../../backbone/utils';


export default {
  props: {
    options: {
      type: Object,
      default:  {
        term: '',
        results: [],
        sortBy: '',
        sortBySelected: '',
      },
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();
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
      };
    },
  },
  methods: {
    changeSortBy (event) {
      this.$emit('changeSortBy', { sortBy: event.target.value });
    },

    selected (val, key) {
      let selected = false;
      if (this.options.sortBySelected) {
        selected = key === this.options.sortBySelected;
      } else {
        selected = val.default;
      }
      return selected;
    }
  }
}
</script>
<style lang="scss" scoped></style>
