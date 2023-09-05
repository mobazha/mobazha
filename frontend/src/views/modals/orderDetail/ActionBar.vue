<template>
  <div class="actionBar gutterV">
    <div v-if="ob.showDisputeOrderButton">
      <ProcessingButton className="flex btn clrErr clrBrDec1 clrTOnEmph"
        :btnText="ob.polyT('orderDetail.actionBar.disputeOrderBtn')" @click="onClickOpenDispute" />
    </div>
  </div>
</template>

<script>
import _ from 'underscore';


export default {
  mixins: [],
  data () {
    return {
      showDisputeOrderButton: false,
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide the order id.');
      }

      this.orderID = options.orderID;
      this._state = {
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
        this.render();
      }

      return this;
    },

  }
}
</script>
<style lang="scss" scoped></style>
