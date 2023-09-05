<template>
  <div class="resolveDisputeTab">
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
          <div class="clrT2 tx6">{{ buyerProfile?.get('name') }}</div>
        </div>
        <div class="col9">
          <FormError v-if="errors['buyerPercentage']" :errors="errors['buyerPercentage']" />
          <div class="flex gutterH">
            <div class="inputBuyerAmountWrap flexNoShrink">
              <input type="text" class="clrBr clrSh2" name="buyerPercentage" id="resolveDisputeBuyerAmount" :value="disputeInfo.buyerPercentage" placeholder="" data-var-type="number" />
              <div class="avatar disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(buyerProfile?.get('avatarHashes').toJSON())"></div>
            </div>
            <p class="buyerContractUnarrivedMsg"><i class="ion-alert-circled margRSm clrTAlert"></i>{{ ob.polyT(`orderDetail.resolveDisputeTab.buyerContractUnavailable`) }} <span class="toolTip clrT" :data-tip="ob.polyT(`orderDetail.resolveDisputeTab.buyerContractUnavailableTip`)"><i class="ion-help-circled"></i></span></p>
          </div>
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="resolveDisputeVendorAmount" class="required rowTn">{{ ob.polyT(`orderDetail.resolveDisputeTab.vendorAmountLabel`) }}</label>
          <div class="clrT2 tx6 js-vendorName">{{ vendorProfile?.get('name') }}</div>
        </div>
        <div class="col9">
          <FormError v-if="errors['vendorPercentage']" :errors="errors['vendorPercentage']" />
          <div class="flex gutterH">
            <div class="inputVendorAmountWrap js-inputVendorWrap flexNoShrink">
              <input type="text" class="clrBr clrSh2" name="vendorPercentage" id="resolveDisputeVendorAmount" :value="disputeInfo.vendorPercentage" placeholder="" data-var-type="number" />
              <div class="avatar disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(vendorProfile?.get('avatarHashes').toJSON())">
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
            :placeholder="ob.polyT(`orderDetail.resolveDisputeTab.commentPlaceholder`)" v-model="disputeInfo.resolution" />
        </div>
      </div>
    </form>
    <hr class="clrBr" />
    <div class="buttonBar flexHRight flexVCent gutterHLg">
      <a class="js-cancel" :disabled="disputeInfo.resolvingDispute" @click="onClickCancel">{{ ob.polyT(`orderDetail.resolveDisputeTab.btnCancel`) }}</a>
      <div class="posR">
        <ProcessingButton
          :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph js-submit ${disputeInfo.resolvingDispute ? 'processing' : ''}`"
          :btnText="ob.polyT(`orderDetail.resolveDisputeTab.btnSubmit`)" @click="onClickSubmit" />
        <div class="js-resolveConfirm confirmBox resolveConfirm tx5 arrowBoxBottom clrBr clrP clrT"
          :hidden="!disputeInfo.resolveConfirmOn" @click="onClickResolveConfirmBox">
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
  },
  data () {
    return {
      disputeInfo: {
        buyerPercentage: 0,
        vendorName: '',
        vendorPercentage: 0,
        resolution: '',
        resolvingDispute: false,
        resolveConfirmOn: false,
      }
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
    errors() {
      return this.model.validationError || {};
    },

    resolvingDispute() {
      return resolvingDispute(this.model.id);
    },
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide an ResolveDispute model.');
      }

      if (!options.case) {
        throw new Error('Please provide a Case model.');
      }

      this.case = options.case;

      checkValidParticipantObject(options.buyer, 'buyer');
      checkValidParticipantObject(options.vendor, 'vendor');

      options.buyer.getProfile().done(profile => {
        this.buyerProfile = profile;
      });

      options.vendor.getProfile().done(profile => {
        this.vendorProfile = profile;
      });

      this.listenTo(orderEvents, 'resolvingDispute', this.onResolvingDispute);
      this.listenTo(orderEvents, 'resolveDisputeComplete resolveDisputeFail',
        this.onResolveDisputeAlways);
      this.listenTo(this.case, 'otherContractArrived', (md, data) => {
        const type = data.isBuyer ? 'buyer' : 'vendor';
        this.$el.toggleClass(`${type}ContractUnavailable`, !this.case.get(`${type}Contract`));
      });

      this.boundOnDocClick = this.onDocumentClick.bind(this);
      $(document).on('click', this.boundOnDocClick);
    },

    onClickResolveConfirmBox () {
      // ensure event doesn't bubble so onDocumentClick doesn't
      // close the confirmBox.
      return false;
    },

    onClickCancelConfirm () {
      recordEvent('OrderDetails_DisputeResolveConfirmCancel');
      $('.js-resolveConfirm').addClass('hide');
    },

    onDocumentClick () {
      $('.js-resolveConfirm').addClass('hide');
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
      $('.js-resolveConfirm').removeClass('hide');
      recordEvent('OrderDetails_DisputeResolveSubmit');
      return false;
    },

    onClickConfirmedSubmit () {
      const formData = this.getFormData();
      this.model.set(formData);
      this.model.set({
        buyerPercentage: this.case.get('buyerContract') ? formData.buyerPercentage : 0,
        vendorPercentage: this.case.get('vendorContract') ? formData.vendorPercentage : 0,
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
        $('.js-submit').addClass('processing');
        $('.js-cancel').addClass('disabled');
      }
    },

    onResolveDisputeAlways (e) {
      if (e.id === this.model.id) {
        $('.js-submit').removeClass('processing');
        $('.js-cancel').removeClass('disabled');
      }
    },

    remove () {
      $(document).off('click', this.boundOnDocClick);
    },

    render () {
      this.$el.toggleClass('vendorContractUnavailable', !this.case.get('vendorContract'));
      this.$el.toggleClass('buyerContractUnavailable', !this.case.get('buyerContract'));
      this.$el.toggleClass('vendorProcessingError', this.case.vendorProcessingError);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
