<template>
  <div class="disputeStartedEvent rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.disputeStarted.heading') }}</h2>
    <template v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ ob.moment(ob.timestamp).format('lll') }}</span>
    </template>
    <div class="border clrBr padMd">
      <div class="flex gutterH clrT">
        <div class="statusIconCol"><span class="clrBr ion-alert-circled"></span></div>
        <div class="flexExpand tx5">
          <div class="rowTn txB">{{ introLine }}</div>
          <div v-html="ob.reason || ob.polyT('orderDetail.summaryTab.disputeStarted.noReasonProvided')"></div>
        </div>
        <template v-if="ob.showResolveButton">
          <div class="col">
            <ProcessingButton className="btn clrBAttGrad clrBrDec1 clrTOnEmph tx5b"
              :btnText="ob.polyT('orderDetail.summaryTab.disputeStarted.resolveBtn')" @click="onClickResolveDispute" />
          </div>
        </template>
      </div>
    </div>

  </div>
</template>

<script>
import _ from 'underscore';
import moment from 'moment';
import {
  events as orderEvents,
} from '../../../../../backbone/utils/order';

export default {
  props: {
    options: {
      type: Object,
      default: {
        disputerName: '',
        claim: '',
      },
    },
  },
  data () {
    return {
      _state: {
        showResolveButton: false,
      }
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    introLine () {
      const ob = this.ob;

      return ob.disputerName ?
        ob.polyT('orderDetail.summaryTab.disputeStarted.partyIsDisputing', { name: ob.disputerName }) :
        ob.polyT('orderDetail.summaryTab.disputeStarted.genericIsDisputed');
    },
    ob() {
      return {
        ...this.templateHelpers,
        ...this.options,
        ...this._state,
        moment,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      this._state = {
        showResolveButton: false,
        ...options.initialState || {},
      };
      this.listenTo(orderEvents, 'resolveDisputeComplete', () => {
        this.setState({ showResolveButton: false, });
      });
    },

    onClickResolveDispute () {
      this.$emit('clickResolveDispute');
    },
  }
}
</script>
<style lang="scss" scoped></style>
