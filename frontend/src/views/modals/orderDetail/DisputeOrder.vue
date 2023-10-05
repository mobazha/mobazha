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
            rows="6"
            name="claim"
            ref="clamTextAread"
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
      <a :disabled="ob.openingDispute" @click="onClickCancel">{{ ob.polyT('orderDetail.disputeOrderTab.btnCancel') }}</a>
      <ProcessingButton
        :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph ${ob.openingDispute ? 'processing' : ''}`"
        :btnText="ob.polyT(`orderDetail.fulfillOrderTab.btnSubmit`)" @click="onClickSubmit" />
    </div>
  </div>
</template>

<script>
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
    bb: Function,
  },
  data () {
    return {
      claim: '',
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();

    this.$refs.clamTextAread.focus();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._model,
        errors: this._model.validationError || {},
        openingDispute: !!openingDispute(this.model.id),
        timeoutMessage: this.options.timeoutMessage,
      };
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
      this.baseInit(options);

      if (!this.model) {
        throw new Error('Please provide a DisputeOrder model.');
      }

      checkValidParticipantObject(options.moderator, 'moderator');

      options.moderator.getProfile()
        .done((modProfile) => {
          this.modProfile = modProfile;
        });

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
      this.render();
      this.$emit('clickCancel');
      recordEvent('OrderDetails_DisputeSubmitCancel');
    },

    onClickSubmit () {
      this.model.set({ claim: this.claim }, { validate: true });

      if (!this.model.validationError) {
        recordEvent('OrderDetails_DisputeSubmit');
        openDispute(this.model.id, this.model.toJSON());
      }

      this.render();
      const $firstErr = $('.errorList:first');
      if ($firstErr.length) $firstErr[0].scrollIntoViewIfNeeded();
    },

    onOpeningDisputeOrder (e) {
      if (e.id === this.model.id) {
        this.openingDispute = true;
      }
    },

    onOpenDisputeAlways (e) {
      if (e.id === this.model.id) {
        this.openingDispute = false;
      }
    },

    render () {
      this.openingDispute = !!openingDispute(this.model.id);

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
