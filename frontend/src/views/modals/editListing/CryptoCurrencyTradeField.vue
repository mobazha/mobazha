<template>
  <div class="cryptoCurrencyTradeField">
    <div class="posR">
      <template v-if="ob.isFetching">
        <SpinnerSVG className="center spinnerMd" />
      </template>
      <div v-show="!ob.isFetching">
        <select id="editListingCoinType" @change="onChangeCoinType(val)" :name="metadata.coinType"
          class="clrBr clrP clrSh2" style="width: 100%">
          <template v-for="(coin, j) in ob.curs" :key="j">
            <option :value="coin.code" :selected="coin.code === ob.selected">{{ coin.name }}</option>
          </template>
        </select>
        <div class="clrT2 txSm helper">{{ ob.polyT('editListing.cryptoCurrencyType.helperCoinType') }}</div>
      </div>
    </div>
  </div>
</template>

<script>
// import app from '../../../app';
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
      select2Opts: {},

      _state: {
        isFetching: false,
        curs: [],
        selected: undefined,
      }
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
    $('#editListingCoinType').select2(this.select2Opts);
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
        select2Opts: {},
        ...options,
        initialState: {
          isFetching: false,
          curs: [],
          selected: undefined,
          ...options.initialState,
        },
      };

      this.baseInit(opts);
    },

    onChangeCoinType (val) {
      this.setState({ selected: val, },);
    },

  }
}
</script>
<style lang="scss" scoped></style>
