<template>
  <div class="actionBar gutterV">
    <template v-if="ob.showDisputeOrderButton">
      <ProcessingButton className="flex btn clrErr clrBrDec1 clrTOnEmph"
        :btnText="ob.polyT('orderDetail.actionBar.disputeOrderBtn')" @click="onClickOpenDispute" />
    </template>
  </div>
</template>

<script>
import _ from 'underscore';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        showDisputeOrderButton: false,
      }
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
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
      this.baseInit(options);

      if (!options.orderID) {
        throw new Error('Please provide the order id.');
      }

      this.orderID = options.orderID;
      this._state = {
        showDisputeOrderButton: false,
        ...options.initialState || {},
      };
    },

    onClickOpenDispute () {
      this.$emit('clickOpenDispute');
    },

    getState () {
      return this._state;
    },

    setState (state, replace = false, renderOnChange = true) {
      let newState;

      if (replace) {
        this._state = {};
      } else {
        newState = _.extend({}, this._state, state);
      }

      if (renderOnChange && !_.isEqual(this._state, newState)) {
        this._state = newState;
      }

      return this;
    },

  }
}
</script>
<style lang="scss" scoped></style>
