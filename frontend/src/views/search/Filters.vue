<template>
  <div class="filters">
    <form class="flexColWide gutterV">
      <template v-for="(filter, key) in ob.filters" :key="key">
        <template v-if="['dropdown', 'checkbox', 'radio'].includes(filter.type)">
          <div class="contentBox pad clrP clrBr clrSh2">
            <div class="rowSm txB tx5b">{{ filter.label }}</div>
            <template v-if="filter.type === 'dropdown'">
              <select :name="key" class="select2Small">
                <!-- // if any option has a selected value use the first one. Otherwise use the first default.
          // parsing the label for emojis isn't needed here because the select list is replaced by select2.js -->
                <option v-for="(option, ind) in filter.options" :key="ind" :selected="selectedIndex(filter) === ind"
                  :value="option.value">{{ option.label }}</option>
              </select>
            </template>

            <template v-else-if="filter.type === 'radio'">
              <div class="flexCol">
                <!-- // if any options has a checked value, check the first one. Otherwise use the first default. -->
                <template v-for="(option, ind) in filter.options" :key="ind">
                  <div class="btnRadio clrBr">
                    <input type="radio" :name="key" :id="key + ind" :checked="selectedIndex(filter) === ind"
                      :value="option.value">
                    <label :for="key + ind"><span v-html="ob.parseEmojis(option.label)"></span></label>
                  </div>
                </template>
              </div>
            </template>

            <template v-else-if="filter.type === 'checkbox'">
              <div class="flexCol checkboxCol row">
                <template v-for="(option, index) in filter.options" :key="index">
                  <input type="checkbox" :checked="isChecked(filter)" :id="key + index"
                    :name="`${key}${filter.options.length > 1 ? '[]' : ''}`" :value="option.value">
                  <label :for="key + index"><span v-html="ob.parseEmojis(option.label)"></span></label>
                </template>
              </div>
              <div class="flex tx5b">
                <a class="flexExpand " @click="clickFilterAll" :name="key">Select All</a>
                <a class="flexExpand txRgt " @click="clickFilterNone" :name="key">Select None</a>
              </div>
              <!-- else { console.log(`Unrecognized filter type: ${filter.type}`); } -->
            </template>
          </div>
        </template>
      </template>
    </form>

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
    this.render();
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
        initialState: {
          filters: {},
          ...options.initialState,
        },
        ...options,
      };

      this.baseInit(opts);
    },

    selectedIndex (filter) {
      let selectedIndex = filter.options.findIndex(opt => opt.checked);
      selectedIndex = selectedIndex === -1 ? filter.options.findIndex(opt => opt.default) : selectedIndex;

      return selectedIndex;
    },

    isChecked (filter) {
      const anyChecked = filter.options.filter(opt => opt.checked);

      let checked = false;
      // if none of the checkboxes have a checked value, use the default values
      if (option.checked || !anyChecked.length && option.default) {
        checked = true;
      }

      return checked;
    },

    events () {
      return {
        'change select': 'changeFilter',
        'change input': 'changeFilter',
      };
    },

    retrieveFormData () {
      return this.getFormData(this.$filters);
    },

    changeFilter () {
      this.trigger('filterChanged', this.retrieveFormData());
    },

    makeFilterAllOrNone (name, all = true) {
      const filters = this.getState().filters;
      const processedData = filters[name];
      processedData.options.forEach((opt, i) => {
        processedData.options[i].checked = all;
      });
      filters[name] = processedData;
      this.setState({ filters });
      this.render(); // The shallow compare in setState won't recognize the filters changed;
      this.changeFilter();
    },

    clickFilterAll (e) {
      this.makeFilterAllOrNone($(e.target).prop('name'), true);
    },

    clickFilterNone (e) {
      this.makeFilterAllOrNone($(e.target).prop('name'), false);
    },

    render () {
      $('select').select2({
        minimumResultsForSearch: 10,
        templateResult: selectEmojis,
        templateSelection: selectEmojis,
      });
      this.$filters = this.$el.find('select, input');

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
