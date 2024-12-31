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
        <Select2 id="sortBy" class="select2Small" v-model="sortBySelected" @change="changeSortBy($event)"
          :options="{
            minimumResultsForSearch: Infinity, // disables the search box
            templateResult: selectEmojis,
            templateSelection: selectEmojis,
          }">
          <option v-for="(val, key) in options.sortBy" :value="key" :selected="selected(val, key)">{{ val.label }}
          </option>
        </Select2>
      </div>
    </template>

  </div>
</template>

<script>
import { selectEmojis } from '../../../backbone/utils';


export default {
  props: {
    options: {
      type: Object,
      default:  {
        term: '',
        results: [],
        sortBy: [],
        sortBySelected: '',
      },
    },
  },
  data () {
    return {
      sortBySelected: '',
    };
  },
  created () {
    this.initEventChain();

    this.sortBySelected = this.options.sortBySelected;
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
      };
    },
  },
  methods: {
    selectEmojis,

    changeSortBy (event) {
      this.$emit('changeSortBy', { sortBy: event.target.value });
    },

    selected (val, key) {
      return this.sortBySelected ? key === this.sortBySelected : val.default;
    }
  }
}
</script>
<style lang="scss" scoped></style>
