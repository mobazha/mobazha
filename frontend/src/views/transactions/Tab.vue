<template>
  <div :class="`${type} tx5`">
    <h2 class="tabHeading txUnb">{{ ob.polyT(`transactions.${type}.heading`) }}</h2>
    <div class="searchWrapper rowMd">
      <input type="text" class="ctrl clrP clrBr clrSh2" @keyup="onKeyUpSearch(filter.search)"
        :placeholder="ob.polyT(`transactions.placeholderSearch${ob.capitalize(type)}`)" v-model="filter.search">
    </div>

    <Filters />
    {{ ob.filtersHtml }}

    <div class="flexVCent row gutterH">
      <div class="flexNoShrink gutterHSm js-queryTotalWrapper" v-show="showTotalWrapper">
        <span class="flexNoShrink js-queryTotalLine">{{ queryTotalLine }}</span>
        <a v-show="currentFilterIsDefault" @click="onClickResetQuery">{{ ob.polyT(`transactions.resetFilters`) }}</a>
      </div>
      <div class="tx6 flexVCent">
        <label class="clrT2 marginLAuto margRSm">{{ ob.polyT('transactions.sort.label') }}</label>
        <select class="tx6 select2Small" @change="onChangeSortBy(filter.sortBy)" style="width: 150px">
          <option value="UNREAD" :selected="filter.sortBy === 'UNREAD'">{{ ob.polyT('transactions.sort.unread') }}</option>
          <option value="DATE_ASC" :selected="filter.sortBy === 'DATE_ASC'">{{ ob.polyT('transactions.sort.dateAsc') }}</option>
          <option value="DATE_DESC" :selected="filter.sortBy === 'DATE_DESC'">{{ ob.polyT('transactions.sort.dateDesc') }}</option>
        </select>
        <div class="select2Small js-sortBySelectDropdownContainer"></div>
      </div>
    </div>

    <div class="js-tableContainer"></div>

  </div>
</template>

<script>
import _ from 'underscore';
import $ from 'jquery';
import app from '../../../backbone/app';
import loadTemplate from '../../../backbone/utils/loadTemplate';
import TransactionsTable from './table/Table';
import { capitalize } from '../../../backbone/utils/string';
import Filters from './Filters.vue'

export default {
  components: {
    Filters,
  },
  mixins: [],
  props: {
  },
  data () {
    return {
      showTotalWrapper: false,
      queryTotalLine: '',
      filter: {},
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
  },
  watch: {
    filter(newVal, oldVal) {
      if (!_.isEqual(newVal, oldVal)) {
        if (this.table) {
          this.table.filterParams = newVal;
        }
      }
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        defaultFilter: {
          search: '',
          sortBy: 'UNREAD',
          states: [],
        },
        ...options,
      };

      opts.initialFilter = opts.initialFilter || { ...opts.defaultFilter };

      if (!this.collection) {
        throw new Error('Please provide a transactions collection.');
      }

      const types = ['sales', 'purchases', 'cases'];

      if (types.indexOf(opts.type) === -1) {
        throw new Error(`Type needs to be one of ${types}.`);
      }

      if (!opts.filterConfig) {
        throw new Error('Please provide a filter config object.');
      }

      if (typeof opts.openOrder !== 'function') {
        throw new Error('Please provide a function to open the order detail modal.');
      }

      this.options = opts || {};
      this.type = opts.type;
      this.filterConfig = opts.filterConfig;
      this._filter = { ...opts.initialFilter };

      this.listenTo(this.collection, 'request', (cl, xhr) => {
        if (this.table) {
          this.showTotalWrapper = false;
        }

        setTimeout(() => {
          if (this.table) {
            xhr.done(data => {
              const count = app.polyglot.t(`transactions.${this.type}.countTransactions`, { smart_count: data.queryCount });
              const countInfo = `<span class="txB">${count}</span>`;
              this.queryTotalLine = app.polyglot.t(`transactions.${this.type}.countTransactionsFound`, { smart_count: countInfo })
              this.showTotalWrapper = true;
            });
          }
        });
      });
    },

    events () {
      return {
        'change .filter input': 'onChangeFilter',
      };
    },

    onChangeFilter () {
      let states = [];
      this.$filterCheckboxes.filter(':checked')
        .each((index, checkbox) => {
          states = states.concat($(checkbox).data('state'));
        });

      this.filter = {
        ...this.filter,
        states,
      };
    },

    onKeyUpSearch (val) {
      // wait until they stop typing
      clearTimeout(this.searchKeyUpTimer);

      this.searchKeyUpTimer = setTimeout(() => {
        this.filter = {
          ...this.filter,
          search: val,
        };
      }, 200);
    },

    onChangeSortBy (val) {
      this.filter = {
        ...this.filter,
        sortBy: val,
      };
    },

    onAttach () {
      if (typeof this.table.onAttach === 'function') {
        this.table.onAttach.call(this.table);
      }
    },

    onClickResetQuery () {
      this.filter = { ...this.options.defaultFilter };
      this.render();
    },

    /*
     * Based on the provided list of checkedStates, this function
     * will return a filterConfig list with the checked value set for each
     * filter.
     */
    setCheckedFilters (filterConfig = [], checkedStates = []) {
      const checkedConfig = [];

      filterConfig.forEach((filter, index) => {
        if (!filter.targetState || !filter.targetState.length) {
          throw new Error(`Filter at index ${index} needs a tragetState ` +
            'provided as an array.');
        }

        filter.targetState.forEach(targetState => {
          checkedConfig[index] = {
            ...filterConfig[index],
            checked: checkedStates.indexOf(targetState) > -1,
          };
        });
      });

      return checkedConfig;
    },

    currentFilterIsDefault () {
      return _.isEqual(this.options.defaultFilter, _.omit(this.filter, 'orderID'));
    }

  get $filterCheckboxes () {
      return this._$filterCheckboxes ||
        (this._$filterCheckboxes = $('.filter input'));
    },

    remove () {
      clearTimeout(this.searchKeyUpTimer);
    },

    render () {
      loadTemplate('transactions/filters.html', (filterT) => {
        const filtersHtml = filterT({
          filters: this.setCheckedFilters(this.filterConfig, this.filter.states),
        });

        loadTemplate('transactions/tab.html', (t) => {
          this.$el.html(t({
            filtersHtml, capitalize,
          }));

          this._$filterCheckboxes = null;

          $('.js-sortBySelect').select2({
            minimumResultsForSearch: -1,
            dropdownParent: $('.js-sortBySelectDropdownContainer'),
          });

          if (this.table) this.table.remove();
          this.table = this.createChild(TransactionsTable, {
            type: this.type,
            collection: this.collection,
            initialFilterParams: this.filter,
            getProfiles: this.options.getProfiles,
            openOrder: this.options.openOrder,
            openedOrderModal: this.options.openedOrderModal,
          });
          $('.js-tableContainer').html(this.table.render().el);
        });
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
