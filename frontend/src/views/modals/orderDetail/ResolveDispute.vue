<template>
  <div
    :class="`resolveDisputeTab ${buyerContractUnavailable ? 'buyerContractUnavailable' : ''} ${vendorContractUnavailable ? 'vendorContractUnavailable' : ''} ${ this.case.vendorProcessingError ? 'vendorProcessingError' : ''}`"
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
          <div class="clrT2 tx6">{{ buyerProfile.name }}</div>
        </div>
        <div class="col9">
          <FormError v-if="errors['buyerPercentage']" :errors="errors['buyerPercentage']" />
          <div class="flex gutterH">
            <div class="inputBuyerAmountWrap flexNoShrink">
              <input type="text" class="clrBr clrSh2" name="buyerPercentage" id="resolveDisputeBuyerAmount" v-model="buyerPercentage" placeholder="" data-var-type="number" />
              <div class="avatar disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(buyerProfile.avatarHashes)"></div>
            </div>
            <p class="buyerContractUnarrivedMsg"><i class="ion-alert-circled margRSm clrTAlert"></i>{{ ob.polyT(`orderDetail.resolveDisputeTab.buyerContractUnavailable`) }} <span class="toolTip clrT" :data-tip="ob.polyT(`orderDetail.resolveDisputeTab.buyerContractUnavailableTip`)"><i class="ion-help-circled"></i></span></p>
          </div>
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="resolveDisputeVendorAmount" class="required rowTn">{{ ob.polyT(`orderDetail.resolveDisputeTab.vendorAmountLabel`) }}</label>
          <div class="clrT2 tx6 js-vendorName">{{ vendorProfile.name }}</div>
        </div>
        <div class="col9">
          <FormError v-if="errors['vendorPercentage']" :errors="errors['vendorPercentage']" />
          <div class="flex gutterH">
            <div class="inputVendorAmountWrap js-inputVendorWrap flexNoShrink">
              <input type="text" class="clrBr clrSh2" name="vendorPercentage" id="resolveDisputeVendorAmount" v-model="vendorPercentage" placeholder="" data-var-type="number" />
              <div class="avatar disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(vendorProfile.avatarHashes)">
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
          <FormError v-if="errors['resolution']" :errors="errors['resolution']" />
          <textarea rows="6" name="resolution" class="clrBr clrP clrSh2" id="resolveDisputeComment"
            :placeholder="ob.polyT(`orderDetail.resolveDisputeTab.commentPlaceholder`)" v-model="resolution" />
        </div>
      </div>
    </form>
    <hr class="clrBr" />
    <div class="buttonBar flexHRight flexVCent gutterHLg">
      <a class="js-cancel" :disabled="resolvingDispute" @click="onClickCancel">{{ ob.polyT(`orderDetail.resolveDisputeTab.btnCancel`) }}</a>
      <div class="posR">
        <ProcessingButton
          :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph js-submit ${resolvingDispute ? 'processing' : ''}`"
          :btnText="ob.polyT(`orderDetail.resolveDisputeTab.btnSubmit`)" @click="onClickSubmit" />
        <div class="js-resolveConfirm confirmBox resolveConfirm tx5 arrowBoxBottom clrBr clrP clrT"
          v-show="resolveConfirmOn" @click="onClickResolveConfirmBox">
          <div class="tx3 txB rowSm">{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.title') }}</div>
          <p>{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.body') }}</p>
          <hr class="clrBr row" />
          <div class="flexHRight flexVCent gutterHLg buttonBar">
            <a @click="onClickCancelConfirm">{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.btnCancel') }}</a>
            <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click="onClickConfirmedSubmit">{{ ob.polyT('orderDetail.resolveDisputeTab.resolveConfirm.btnSubmit') }}</a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import {
  resolvingDispute,
  resolveDispute,
  events as orderEvents,
} from '../../../../backbone/utils/order';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { checkValidParticipantObject } from '../../../utils/utils';


export default {
  mixins: [],
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      case: {},
      buyerProfile: {},
      vendorProfile: {},
      buyerPercentage: 0,
      vendorPercentage: 0,
      resolution: '',
      resolvingDispute: false,
      resolveConfirmOn: false,

      buyerContractUnavailable: true,
      vendorContractUnavailable: true,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    errors() {
      return this.model.validationError || {};
    },
  },
  methods: {
    loadData (options = {}) {
      this.model = options.model;
      if (!this.model) {
        throw new Error('Please provide an ResolveDispute model.');
      }

      if (!options.case) {
        throw new Error('Please provide a Case model.');
      }

      this.resolvingDispute = resolvingDispute(this.model.id);

      this.case = options.case;

      checkValidParticipantObject(options.buyer, 'buyer');
      checkValidParticipantObject(options.vendor, 'vendor');

      options.buyer.getProfile().done(profile => {
        this.buyerProfile = profile?.toJSON() || {};
      });

      options.vendor.getProfile().done(profile => {
        this.vendorProfile = profile?.toJSON() || {};
      });

      this.listenTo(orderEvents, 'resolvingDispute', this.onResolvingDispute);
      this.listenTo(orderEvents, 'resolveDisputeComplete resolveDisputeFail', this.onResolveDisputeAlways);
      this.listenTo(this.case, 'otherContractArrived', () => {
        this.buyerContractUnavailable = !this.case.get('buyerContract');
        this.vendorContractUnavailable = !this.case.get('vendorContract');
      });
    },

    onClickResolveConfirmBox () {
      // ensure event doesn't bubble so onDocumentClick doesn't
      // close the confirmBox.
      return false;
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
      this.render();
      this.$emit('clickCancel');
      recordEvent('OrderDetails_DisputeResolveCancel');
    },

    onClickSubmit () {
      this.resolveConfirmOn = true;
      recordEvent('OrderDetails_DisputeResolveSubmit');
      return false;
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

      this.render();
      const $firstErr = $('.errorList:first');
      if ($firstErr.length) $firstErr[0].scrollIntoViewIfNeeded();
    },

    onResolvingDispute (e) {
      if (e.id === this.model.id) {
        this.resolvingDispute = true;
      }
    },

    onResolveDisputeAlways (e) {
      if (e.id === this.model.id) {
        this.resolvingDispute = false;
      }
    },

    render () {
      this.buyerContractUnavailable = !this.case.get('buyerContract');
      this.vendorContractUnavailable = !this.case.get(`vendorContract`);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
