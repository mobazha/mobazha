<template>
  <div :class="className">
    <h2 class="tabHeading txUnb">{{ ob.polyT(`transactions.${ob.type}.heading`) }}</h2>
    <div class="searchWrapper rowMd">
      <input
        type="text"
        class="ctrl clrP clrBr clrSh2"
        @keyup="onKeyUpSearch(filter.search)"
        :placeholder="ob.polyT(`transactions.placeholderSearch${ob.capitalize(ob.type)}`)"
        v-model="filter.search"
      />
    </div>

    <Filters
      :className="className"
      :filters="setCheckedFilters(filterConfig, filter.states)"
      @changeFilter="onChangeFilter"/>

    <div class="flexVCent row gutterH">
      <div class="flexNoShrink gutterHSm js-queryTotalWrapper" v-show="collection.length">
        <span class="flexNoShrink js-queryTotalLine" v-html="queryTotalLine"></span>
        <a v-show="!currentFilterIsDefault" @click="onClickResetQuery">{{ ob.polyT(`transactions.resetFilters`) }}</a>
      </div>
      <div class="tx6 flexVCent">
        <label class="clrT2 marginLAuto margRSm">{{ ob.polyT('transactions.sort.label') }}</label>
        <Select2 class="tx6 select2Small js-sortBySelect" :options="{ minimumResultsForSearch: -1, }" v-model="filter.sortBy" style="width: 150px">
          <option value="UNREAD" :selected="ob.filter.sortBy === 'UNREAD'">{{ ob.polyT('transactions.sort.unread') }}</option>
          <option value="DATE_ASC" :selected="ob.filter.sortBy === 'DATE_ASC'">{{ ob.polyT('transactions.sort.dateAsc') }}</option>
          <option value="DATE_DESC" :selected="ob.filter.sortBy === 'DATE_DESC'">{{ ob.polyT('transactions.sort.dateDesc') }}</option>
        </Select2>
      </div>
    </div>

    <TransactionsTable ref="table" :key="filterKey" :options="tableOptions"
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
      filterKey: 0,

      defaultFilter: {
        search: '',
        sortBy: 'UNREAD',
        states: [],
      },

      filter: {
        search: '',
        sortBy: 'UNREAD',
        states: [],
      },
      showTotalWrapper: false,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
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
          capitalize,
      };
    },
    className () {
      return `${this.type} tx5`;
    },
    currentFilterIsDefault() {
      return _.isEqual(this.defaultFilter, _.omit(this.filter, 'orderID'));
    },
    tableOptions() {
      return {
        type: this.type,
        filterParams: this.filter,
        getProfiles: this.options.getProfiles,
      };
    },
    queryTotalLine() {
      const count = app.polyglot.t(`transactions.${this.type}.countTransactions`, { smart_count: this.collection.length });
      const countInfo = `<span class="txB">${count}</span>`;
      return app.polyglot.t(`transactions.${this.type}.countTransactionsFound`, { smart_count: countInfo });
    }
  },
  watch: {
    filter: {
      handler() {
        this.filterKey += 1;
      },
      deep: true,
    }
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
        this.filter.states.sort((a, b) => a - b);
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
  },
};
</script>
<style lang="scss" scoped></style>
