<template>
  <div
    :class="`resolveDisputeTab ${buyerContractUnavailable ? 'buyerContractUnavailable' : ''} ${vendorContractUnavailable ? 'vendorContractUnavailable' : ''} ${ vendorProcessingError ? 'vendorProcessingError' : ''}`"
    @click="onDocumentClick">
    <div class="padLg flexVCent">
      <div class="backToSummaryWrap">
        <a class="clrTEm txU" @click="onClickBackToSummary">{{ ob.polyT(`orderDetail.backToSummary`) }}</a>
      </div>
      <div class="txCtr txB tx3 flexExpand">{{ ob.polyT(`orderDetail.resolveDisputeTab.heading`) }}</div>
    </div>
    <hr class="clrBr rowMd" />
    <form class="padKids padStack pad clrP clrBr js-fulfillForm">
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="resolveDisputeBuyerAmount" class="required rowTn">{{
            ob.polyT(`orderDetail.resolveDisputeTab.buyerAmountLabel`) }}</label>
          <div class="clrT2 tx6">{{ ob.buyerName }}</div>
        </div>
        <div class="col9">
          <FormError v-if="ob.errors['buyerPercentage']" :errors="ob.errors['buyerPercentage']" />
          <div class="flex gutterH">
            <div class="inputBuyerAmountWrap flexNoShrink">
              <input type="number" class="clrBr clrSh2" name="buyerPercentage" id="resolveDisputeBuyerAmount" v-model="buyerPercentage" placeholder="" data-var-type="number" />
              <div class="avatar disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(ob.buyerAvatarHashes)"></div>
            </div>
            <p class="buyerContractUnarrivedMsg"><i class="ion-alert-circled margRSm clrTAlert"></i>{{ ob.polyT(`orderDetail.resolveDisputeTab.buyerContractUnavailable`) }} <span class="toolTip clrT" :data-tip="ob.polyT(`orderDetail.resolveDisputeTab.buyerContractUnavailableTip`)"><i class="ion-help-circled"></i></span></p>
          </div>
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="resolveDisputeVendorAmount" class="required rowTn">{{ ob.polyT(`orderDetail.resolveDisputeTab.vendorAmountLabel`) }}</label>
          <div class="clrT2 tx6 js-vendorName">{{ ob.vendorName }}</div>
        </div>
        <div class="col9">
          <FormError v-if="ob.errors['vendorPercentage']" :errors="ob.errors['vendorPercentage']" />
          <div class="flex gutterH">
            <div class="inputVendorAmountWrap js-inputVendorWrap flexNoShrink">
              <input type="number" class="clrBr clrSh2" name="vendorPercentage" id="resolveDisputeVendorAmount" v-model="vendorPercentage" placeholder="" data-var-type="number" />
              <div class="avatar disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(ob.vendorAvatarHashes)">
              </div>
            </div>
            <p class="vendorContractUnarrivedMsg">
              <i class="ion-alert-circled margRSm clrTAlert"></i>
              {{ ob.polyT(`orderDetail.resolveDisputeTab.vendorContractUnavailable`) }}
              <span class="toolTip clrT" :data-tip="ob.polyT(`orderDetail.resolveDisputeTab.vendorContractUnavailableTip`)">
                <i class="ion-help-circled"></i>
              </span>
            </p>
            <p class="vendorProcErrContractUnarrivedMsg">
              <i class="ion-alert-circled margRSm clrTAlert"></i>
              {{ ob.polyT(`orderDetail.resolveDisputeTab.vendorContractUnavailableProcErr`) }}
            </p>
          </div>
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="resolveDisputeComment" class="required">{{ ob.polyT(`orderDetail.resolveDisputeTab.commentLabel`) }}</label>
        </div>
        <div class="col7">
          <FormError v-if="ob.errors['resolution']" :errors="ob.errors['resolution']" />
          <textarea rows="6" name="resolution" class="clrBr clrP clrSh2" id="resolveDisputeComment"
            :placeholder="ob.polyT(`orderDetail.resolveDisputeTab.commentPlaceholder`)" v-model="resolution" />
        </div>
      </div>
    </form>
    <hr class="clrBr" />
    <div class="buttonBar flexHRight flexVCent gutterHLg">
      <a class="js-cancel" :disabled="processing" @click="onClickCancel">{{ ob.polyT(`orderDetail.resolveDisputeTab.btnCancel`) }}</a>
      <div class="posR">
        <ProcessingButton
          :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph js-submit ${processing ? 'processing' : ''}`"
          :btnText="ob.polyT(`orderDetail.resolveDisputeTab.btnSubmit`)" @click.stop="onClickSubmit" />
        <div class="js-resolveConfirm confirmBox resolveConfirm tx5 arrowBoxBottom clrBr clrP clrT"
          v-show="resolveConfirmOn" @click.stop.prevent>
          <div class="tx3 txB rowSm">{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.title') }}</div>
          <p>{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.body') }}</p>
          <hr class="clrBr row" />
          <div class="flexHRight flexVCent gutterHLg buttonBar">
            <a @click.stop="onClickCancelConfirm">{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.btnCancel') }}</a>
            <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click.stop="onClickConfirmedSubmit">{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.btnSubmit') }}</a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import ResolveDisputeMd from '../../../../backbone/models/order/ResolveDispute';
import {
  resolvingDispute,
  resolveDispute,
  events as orderEvents,
} from '../../../../backbone/utils/order';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { checkValidParticipantObject } from '../../../utils/utils';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      _model: undefined,
      _modelKey: 0,

      buyerPercentage: 0,
      vendorPercentage: 0,
      resolution: '',

      processing: false,
      resolveConfirmOn: false,

      buyerAvatarHashes: {},
      buyerName: '',

      vendorAvatarHashes: {},
      vendorName: '',
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
      };
    },
    model() {
      let access = this._modelKey;

      return this._model;
    },
    buyerContractUnavailable() {
      return !this._case.buyerContract;
    },
    vendorContractUnavailable() {
      return !this._case.vendorContract;
    },
    vendorProcessingError() {
      let access = this._case;
      return this.case.vendorProcessingError
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.case) {
        throw new Error('Please provide a Case model.');
      }

      checkValidParticipantObject(options.buyer, 'buyer');
      checkValidParticipantObject(options.vendor, 'vendor');

      this.baseInit(options);

      let modelAttrs = { orderID: this.case.id };
      const isResolvingDispute = resolvingDispute(this.case.id);

      // If this order is in the process of the dispute being resolved, we'll
      // populate the model with the data that was posted to the server.
      if (isResolvingDispute) {
        modelAttrs = {
          ...modelAttrs,
          ...isResolvingDispute.data,
        };
      }

      this._model = new ResolveDisputeMd(modelAttrs, {
        buyerContractArrived: () => !!this.case.get('buyerContract'),
        vendorContractArrived: () => !!this.case.get('vendorContract'),
        vendorProcessingError: () => this.case.vendorProcessingError,
      });
      this._model.on('change', () => this._modelKey += 1);

      options.buyer.getProfile().done(profile => {
        this.buyerName = profile.get('name');
        this.buyerAvatarHashes = profile.get('avatarHashes').toJSON();

      });

      options.vendor.getProfile().done(profile => {
        this.vendorName = profile.get('name');
        this.vendorAvatarHashes = profile.get('avatarHashes').toJSON();
      });

      this.processing = resolvingDispute(this._model.id);
      this.listenTo(orderEvents, 'resolvingDispute', this.onResolvingDispute);
      this.listenTo(orderEvents, 'resolveDisputeComplete resolveDisputeFail', this.onResolveDisputeAlways);
    },

    onClickCancelConfirm () {
      recordEvent('OrderDetails_DisputeResolveConfirmCancel');
      this.resolveConfirmOn = false;
    },

    onDocumentClick () {
      this.resolveConfirmOn = false;
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
      recordEvent('OrderDetails_DisputeResolveCancel');
    },

    onClickSubmit () {
      this.resolveConfirmOn = true;
      recordEvent('OrderDetails_DisputeResolveSubmit');
    },

    onClickConfirmedSubmit () {
      this.model.set({
        buyerPercentage: this.case.get('buyerContract') ? this.buyerPercentage : 0,
        vendorPercentage: this.case.get('vendorContract') ? this.vendorPercentage : 0,
        resolution: this.resolution,
      }, { validate: true });

      if (!this.model.validationError) {
        recordEvent('OrderDetails_DisputeResolveConfirm');
        resolveDispute(this.model);
      }

      const $firstErr = $('.errorList:first');
      if ($firstErr.length) $firstErr[0].scrollIntoViewIfNeeded();
    },

    onResolvingDispute (e) {
      if (e.id === this.model.id) {
        this.processing = true;
      }
    },

    onResolveDisputeAlways (e) {
      if (e.id === this.model.id) {
        this.processing = false;
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
