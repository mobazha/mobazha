<template>
  <div class="fulfillOrderTab">
    <div class="padLg flexVCent">
      <div class="backToSummaryWrap">
        <a class="clrTEm txU" @click="onClickBackToSummary">{{ ob.polyT(`orderDetail.backToSummary`) }}</a>
      </div>
      <div class="txCtr txB tx3 flexExpand">{{ ob.polyT(`orderDetail.fulfillOrderTab.heading`) }}</div>
    </div>
    <hr class="clrBr rowMd" />
    <form class="padKids padStack pad clrP clrBr js-fulfillForm">
      <div v-if="contractType === 'PHYSICAL_GOOD' && !isLocalPickup">
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderShippingCarrier" class="required">{{
              ob.polyT(`orderDetail.fulfillOrderTab.shippingCarrierLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="errors['physicalDelivery.shipper']" :errors="errors['physicalDelivery.shipper']" />
            <input type="text" class="clrBr clrSh2" name="physicalDelivery.shipper" id="fulfillOrderShippingCarrier"
              :value="ob.physicalDelivery.shipper"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.shippingCarrierPlaceholder`)" />
          </div>
        </div>
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderTrackingNumber">{{ ob.polyT(`orderDetail.fulfillOrderTab.trackingLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="errors['physicalDelivery.trackingNumber']"
              :errors="errors['physicalDelivery.trackingNumber']" />
            <input type="text" class="clrBr clrSh2" name="physicalDelivery.trackingNumber" id="fulfillOrderTrackingNumber"
              :value="ob.physicalDelivery.trackingNumber"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.trackingPlaceholder`)" />
          </div>
        </div>
      </div>

      <div v-else-if="contractType === 'DIGITAL_GOOD'">
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderFileUrl" class="required">{{ ob.polyT(`orderDetail.fulfillOrderTab.fileUrlLabel`)
            }}</label>
          </div>
          <div class="col7">
            <FormError v-if="errors['digitalDelivery.url']" :errors="errors['digitalDelivery.url']" />
            <input type="text" class="clrBr clrSh2" name="digitalDelivery.url" id="fulfillOrderFileUrl"
              :value="ob.digitalDelivery.url" :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.fileUrlPlaceholder`)" />
          </div>
        </div>
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderPassword">{{ ob.polyT(`orderDetail.fulfillOrderTab.passwordLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="errors['digitalDelivery.password']" :errors="errors['digitalDelivery.password']" />
            <input type="text" class="clrBr clrSh2" name="digitalDelivery.password" id="fulfillOrderPassword"
              :value="ob.digitalDelivery.password"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.passwordPlaceholder`)" />
          </div>
        </div>
      </div>

      <div v-else-if="contractType === 'CRYPTOCURRENCY'">
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderTransactionID" class="required">{{
              ob.polyT(`orderDetail.fulfillOrderTab.transactionIDLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="errors['cryptocurrencyDelivery.transactionID']"
              :errors="errors['cryptocurrencyDelivery.transactionID']" />
            <input type="text" class="clrBr clrSh2" name="cryptocurrencyDelivery.transactionID"
              id="fulfillOrderTransactionID" :value="ob.cryptocurrencyDelivery.transactionID"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.transactionIDPlaceholder`)"
              :maxlength="cryptoDelivery && cryptoDelivery.constraints || {}.transactionIDLength" />
          </div>
        </div>
      </div>
      <div class="flexRow gutterH rowHg">
        <div class="col3">
          <label for="fulfillOrderNote">{{ ob.polyT(`orderDetail.fulfillOrderTab.noteLabel`) }}</label>
        </div>
        <div class="col7">
          <FormError v-if="errors['note']" :errors="errors['note']" />
          <textarea rows="6" name="note" :class="`clrBr clrP clrSh2 ${contractType === 'DIGITAL_GOOD' ? 'rowSm' : ''}`"
            id="fulfillOrderNote" :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.notePlaceholder`)"
            v-model="ob.note"></textarea>
          <div v-if="contractType === 'DIGITAL_GOOD'">
            <div class="clrT2 txSm">{{ ob.polyT(`orderDetail.fulfillOrderTab.noteHelperTextDigital`) }}</div>
          </div>
        </div>
      </div>
    </form>
    <hr class="clrBr" />
    <div class="buttonBar flexHRight flexVCent gutterHLg">
      <a class="js-cancel" :disabled="ob.fulfillingOrder" @click="onClickCancel">{{
        ob.polyT(`orderDetail.fulfillOrderTab.btnCancel`) }}</a>
      <ProcessingButton
        :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph js-submit ${fulfillingOrder(model.id) ? 'processing' : ''}`"
        :btnText="ob.polyT(`orderDetail.fulfillOrderTab.btnSubmit`)" @click="onClickSubmit" />
    </div>
  </div>
</template>

<script>
import {
  fulfillingOrder,
  fulfillOrder,
  events as orderEvents,
} from '../../../../backbone/utils/order';

export default {
  mixins: [],
  props: {
  },
  data () {
    return {
      autoFocusFirstField: true,

      fulfillingOrder: false,
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
    errors () {
      return this.model.validationError || {};
    },
    btnCancel () {
      return this._btnCancel ||
        (this._btnCancel = $('.js-cancel'));
    },

    btnSubmit () {
      return this._btnSubmit ||
        (this._btnSubmit = $('.js-submit'));
    },
    cryptoDelivery () {
      return this.model.get('cryptocurrencyDelivery');
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide an OrderFulfillment model.');
      }

      if (!options.contractType) {
        throw new Error('Please provide the contract type.');
      }

      if (typeof options.isLocalPickup !== 'boolean') {
        throw new Error('Please provide a boolean indicating whether the item is to ' +
          'be picked up locally.');
      }

      this.contractType = options.contractType;
      this.isLocalPickup = options.isLocalPickup;
      this.listenTo(orderEvents, 'fulfillingOrder', this.onFulfillingOrder);
      this.listenTo(orderEvents, 'fulfillOrderComplete, fulfillOrderFail',
        this.onFulfillOrderAlways);
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
    },

    onClickSubmit () {
      const formData = this.getFormData();
      this.model.set(formData);
      this.model.set({}, { validate: true });

      if (!this.model.validationError) {
        fulfillOrder(this.contractType, this.isLocalPickup, this.model.toJSON());
      }

      this.render();
      const $firstErr = $('.errorList:first');
      if ($firstErr.length) $firstErr[0].scrollIntoViewIfNeeded();
    },

    onFulfillingOrder (e) {
      if (e.id === this.model.id) {
        this.btnSubmit.addClass('processing');
        this.btnCancel.addClass('disabled');
      }
    },

    onFulfillOrderAlways (e) {
      if (e.id === this.model.id) {
        this.btnSubmit.removeClass('processing');
        this.btnCancel.removeClass('disabled');
      }
    },


    render () {
      this.fulfillingOrder = fulfillingOrder(this.model.id);

      this._btnCancel = null;
      this._btnSubmit = null;

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
