<template>
  <div class="bulkCoinUpdateBtn flex gutterH" @click="onDocumentClick">
    <template v-if="ob.error">
      <span class="clrTErr">{{ ob.polyT(`settings.storeTab.bulkListingCoinUpdate.errors.${ob.error}`) }}</span>
    </template>
    <div class="flexNoShrink" style="width: 240px">
      <ProcessingButton
        :className="`btn clrP clrBr clrT clrSh2 ${ob.isBulkCoinUpdating ? 'processing' : ''} js-applyToCurrent`"
        @click.stop="clickApplyToCurrent" :btnText="ob.polyT('settings.storeTab.bulkListingCoinUpdate.mainButton')" />
      <template v-if="showConfirmTooltip">
        <div class="confirmBox arrowBoxTop clrBr clrP clrT clrSh1 js-confirmBox" @click.stop.prevent>
          <div class="tx3 txB rowSm">{{ ob.polyT('settings.storeTab.bulkListingCoinUpdate.confirmTitle') }}</div>
          <div class="posR padSm">{{ ob.polyT('settings.storeTab.bulkListingCoinUpdate.confirmMessage') }}</div>
          <hr class="clrBr row" />
          <div class="flexVCent gutterHLg">
            <div class="flexExpand">
              <!-- // The cancel button is just cosmetic, cancelling is handled by the click on the document -->
              <a class="clrT2" @click.stop="clickApplyToCurrentCancel">{{
                ob.polyT('settings.storeTab.bulkListingCoinUpdate.cancel') }}</a>
            </div>
            <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click.stop="clickApplyToCurrentConfirm">{{
              ob.polyT('settings.storeTab.bulkListingCoinUpdate.apply') }}</a>
          </div>
        </div>
      </template>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import {
  isBulkCoinUpdating,
  events as bulkCoinUpdateEvents,
} from '../../../../backbone/utils/bulkCoinUpdate';


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
        isBulkCoinUpdating: false,
        error: '',
      },
      showConfirmTooltip: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  unmounted() {
    clearTimeout(this.processingTimer);
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
        ...options,
        initialState: {
          isBulkCoinUpdating: isBulkCoinUpdating(),
          error: '',
          ...options.initialState,
        },
      };
      this.baseInit(opts);

      this.listenTo(bulkCoinUpdateEvents, 'bulkCoinUpdateDone bulkCoinUpdateFailed',
        () => this.setState({ isBulkCoinUpdating: false }));
    },

    startProcessingTimer () {
      if (!this.processingTimer) {
        this.processingTimer = setTimeout(() => {
          this.processingTimer = null;
          // If the update is still pending, let it set the isBulkCoinUpdating state.
          if (!isBulkCoinUpdating()) this.setState({ isBulkCoinUpdating: false });
        }, 500);
      }
    },

    setState (state = {}) {
      // When the state is set to processing, start a timer so it's visible even if it's very short.
      if (state.isBulkCoinUpdating) this.startProcessingTimer();

      // If the state is set to stop processing, let the timer finish.
      if (state.hasOwnProperty('isBulkCoinUpdating') &&
        !state.isBulkCoinUpdating &&
        this.processingTimer) {
        delete state.isBulkCoinUpdating;
      }

      _.extend(this._state, state);
    },

    clickApplyToCurrent () {
      this.showConfirmTooltip = true;
    },

    clickApplyToCurrentCancel () {
      this.showConfirmTooltip = false;
    },

    clickApplyToCurrentConfirm () {
      this.$emit('bulkCoinUpdateConfirm');
      this.showConfirmTooltip = false;
    },

    onDocumentClick () {
      if (this.showConfirmTooltip) {
        this.showConfirmTooltip = false;
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
