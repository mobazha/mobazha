<template>
  <div class="disputeOrderTab">
    <div class="padLg flexVCent">
      <div class="backToSummaryWrap">
        <a class="clrTEm txU" @click="onClickBackToSummary">{{ ob.polyT(`orderDetail.backToSummary`) }}</a>
      </div>
      <div class="txCtr txB tx3 flexExpand">{{ ob.polyT(`orderDetail.disputeOrderTab.heading`) }}</div>
    </div>
    <hr class="clrBr rowMd" />
    <form class="padKids padStack pad clrP clrBr js-fulfillForm">
      <div class="flexRow gutterH row">
        <div class="col3">
          <div for="fulfillOrderNote">{{ ob.polyT(`orderDetail.disputeOrderTab.moderatorLabel`) }}</div>
        </div>
        <div class="col7">
          <div class="js-modContainer">
            <ModFragment v-if="modProfile"
              :options="moderatorState"
              :bb="function() {
                return {
                  model: modProfile,
                };
              }"
            />
          </div>
        </div>
      </div>
      <div class="flexRow gutterH rowHg">
        <div class="col3">
          <label for="fulfillOrderNote">{{ ob.polyT(`orderDetail.disputeOrderTab.reasonLabel`) }}</label>
        </div>
        <div class="col7">
          <FormError v-if="ob.errors['claim']" :errors="ob.errors['claim']" />
          <textarea
            v-focus
            rows="6"
            name="claim"
            class="clrBr clrP clrSh2 row"
            id="fulfillOrderNote"
            :placeholder="ob.polyT(`orderDetail.disputeOrderTab.reasonPlaceholder`)"
            v-model="claim" />
          <p class="clrT2 txSm">{{ ob.polyT(`orderDetail.disputeOrderTab.reasonHelperText`) }}</p>
          <p v-if="ob.timeoutMessage" class="clrT2 txSm">{{ ob.timeoutMessage }}</p>
        </div>
      </div>
    </form>
    <hr class="clrBr" />
    <div class="buttonBar flexHRight flexVCent gutterHLg">
      <a :disabled="processing" @click="onClickCancel">{{ ob.polyT('orderDetail.disputeOrderTab.btnCancel') }}</a>
      <ProcessingButton
        :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph ${processing ? 'processing' : ''}`"
        :btnText="ob.polyT(`orderDetail.fulfillOrderTab.btnSubmit`)" @click="onClickSubmit" />
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import OrderDispute from '../../../../backbone/models/order/OrderDispute';
import {
  openingDispute,
  openDispute,
  events as orderEvents,
} from '../../../../backbone/utils/order';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { checkValidParticipantObject } from '../../../utils/utils';
import ModFragment from './ModFragment.vue';


export default {
  components: {
    ModFragment,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _model: undefined,
      _modelKey: 0,

      claim: '',

      processing: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        errors: this.model.validationError || {},
        timeoutMessage: this.options.timeoutMessage,
      };
    },
    model() {
      let access = this._modelKey;

      return this._model;
    },
    moderatorState() {
      return {
        peerID: this.options.moderator.id,
        showAvatar: true,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide the orderID.');
      }

      checkValidParticipantObject(options.moderator, 'moderator');

      this.baseInit(options);

      this._model = new OrderDispute({ orderID: options.orderID });
      this._model.on('change', () => this._modelKey += 1);

      options.moderator.getProfile()
        .done((modProfile) => {
          this.modProfile = modProfile;
        });

      this.processing = !!openingDispute(this._model.id);
      this.listenTo(orderEvents, 'openingDispute', this.onOpeningDispute);
      this.listenTo(orderEvents, 'openDisputeComplete, openDisputeFail', this.onOpenDisputeAlways);
    },

    onClickBackToSummary () {
      this.$emit('clickBackToSummary');
    },

    onClickCancel () {
      const id = this.model.id;
      this.model.reset();
      // restore the id reset blew away
      this.model.set({ orderID: id });

      this.$emit('clickCancel');
      recordEvent('OrderDetails_DisputeSubmitCancel');
    },

    onClickSubmit () {
      this.model.set({ claim: this.claim }, { validate: true });

      if (!this.model.validationError) {
        recordEvent('OrderDetails_DisputeSubmit');
        openDispute(this.model.id, this.model.toJSON());
      }

      const firstErr = $('.errorList:first');
      if (firstErr.length) firstErr[0].scrollIntoViewIfNeeded();
    },

    onOpeningDisputeOrder (e) {
      if (e.id === this.model.id) {
        this.processing = true;
      }
    },

    onOpenDisputeAlways (e) {
      if (e.id === this.model.id) {
        this.processing = false;
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
