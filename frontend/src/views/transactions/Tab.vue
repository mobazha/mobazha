<template>
  <div :class="`${ob.type} tx5`">
    <h2 class="tabHeading txUnb">{{ ob.polyT(`transactions.${ob.type}.heading`) }}</h2>
    <div class="searchWrapper rowMd">
      <input
        type="text"
        class="ctrl clrP clrBr clrSh2"
        @keyup="onKeyUpSearch(ob.filter.search)"
        :placeholder="ob.polyT(`transactions.placeholderSearch${ob.capitalize(ob.type)}`)"
      />
    </div>

    <Filters
      :filters="setCheckedFilters(filterConfig, filter.states)"
      @changeFilter="onChangeFilter"/>

    <div class="flexVCent row gutterH">
      <div class="flexNoShrink gutterHSm js-queryTotalWrapper" v-show="collection.length">
        <span class="flexNoShrink js-queryTotalLine" v-html="queryTotalLine"></span>
        <a v-show="currentFilterIsDefault" @click="onClickResetQuery">{{ ob.polyT(`transactions.resetFilters`) }}</a>
      </div>
      <div class="tx6 flexVCent">
        <label class="clrT2 marginLAuto margRSm">{{ ob.polyT('transactions.sort.label') }}</label>
        <select class="tx6 select2Small js-sortBySelect" @change="onChangeSortBy(ob.filter.sortBy)" style="width: 150px">
          <option value="UNREAD" :selected="ob.filter.sortBy === 'UNREAD'">{{ ob.polyT('transactions.sort.unread') }}</option>
          <option value="DATE_ASC" :selected="ob.filter.sortBy === 'DATE_ASC'">{{ ob.polyT('transactions.sort.dateAsc') }}</option>
          <option value="DATE_DESC" :selected="ob.filter.sortBy === 'DATE_DESC'">{{ ob.polyT('transactions.sort.dateDesc') }}</option>
        </select>
        <div class="select2Small js-sortBySelectDropdownContainer"></div>
      </div>
    </div>

    <TransactionsTable ref="table" :options="tableOptions"
      :bb="function() {
        return {
          collection,
        };
      }"
      @clickRow="onClickRow" />
  </div>
</template>

<script>
import _ from 'underscore';
import $ from 'jquery';
import app from '../../../backbone/app';
import { capitalize } from '../../../backbone/utils/string';
import TransactionsTable from './table/Table.vue';
import Filters from './Filters.vue';

export default {
  components: {
    Filters,
    TransactionsTable,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      defaultFilter: {
        search: '',
        sortBy: 'UNREAD',
        states: [],
      },

      filter: {},
      showTotalWrapper: false,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    $('.js-sortBySelect').select2({
      minimumResultsForSearch: -1,
      dropdownParent: $('.js-sortBySelectDropdownContainer'),
    });
  },
  unmounted() {
    clearTimeout(this.searchKeyUpTimer);
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
          type: this.type,
          filter: this.filter,
          currentFilterIsDefault: this.currentFilterIsDefault(),
          capitalize,
      };
    },
    tableOptions() {
      return {
        type: this.type,
        filterParams: this.filter,
        getProfiles: this.options.getProfiles,
      };
    },
    queryTotalLine() {
      console.log('this.collection: ', this.collection)
      const count = app.polyglot.t(`transactions.${this.type}.countTransactions`, { smart_count: this.collection.length });
      const countInfo = `<span class="txB">${count}</span>`;
      return app.polyglot.t(`transactions.${this.type}.countTransactionsFound`, { smart_count: countInfo });
    }
  },
  watch: {
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        defaultFilter: {
          search: '',
          sortBy: 'UNREAD',
          states: [],
        },
        ...options,
      };

      opts.initialFilter = opts.initialFilter || { ...opts.defaultFilter };

      this.baseInit(opts);

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

      this.type = opts.type;
      this.filterConfig = opts.filterConfig;
      this.filter = { ...opts.initialFilter };
    },

    events() {
      return {
        'change .filter input': 'onChangeFilter',
      };
    },

    onClickRow(orderID, type) {
      this.$emit('clickRow', orderID, type);
    },

    onChangeFilter(filter, checked) {
      if (checked) {
        filter.targetState.forEach((targetState) => {
          if (this.filter.states.indexOf(targetState) == -1) {
            this.filter.states.push(targetState);
          }
        });
      } else {
        this.filter.states = this.filter.states.filter((item) => { return filter.targetState.indexOf(item) == -1; });
      }
    },

    onKeyUpSearch(val) {
      // wait until they stop typing
      clearTimeout(this.searchKeyUpTimer);

      this.searchKeyUpTimer = setTimeout(() => {
        this.filter = {
          ...this.filter,
          search: val,
        };
      }, 200);
    },

    onChangeSortBy(val) {
      this.filter = {
        ...this.filter,
        sortBy: val,
      };
    },

    onClickResetQuery() {
      this.filter = { ...this.defaultFilter };
    },

    /*
     * Based on the provided list of checkedStates, this function
     * will return a filterConfig list with the checked value set for each
     * filter.
     */
    setCheckedFilters(filterConfig = [], checkedStates = []) {
      const checkedConfig = [];

      filterConfig.forEach((filter, index) => {
        if (!filter.targetState || !filter.targetState.length) {
          throw new Error(`Filter at index ${index} needs a tragetState ` + 'provided as an array.');
        }

        filter.targetState.forEach((targetState) => {
          checkedConfig[index] = {
            ...filterConfig[index],
            checked: checkedStates.indexOf(targetState) > -1,
          };
        });
      });

      return checkedConfig;
    },

    currentFilterIsDefault() {
      return _.isEqual(this.defaultFilter, _.omit(this.filter, 'orderID'));
    },

    remove() {
      clearTimeout(this.searchKeyUpTimer);
    },
  },
};
</script>
<style lang="scss" scoped></style>
