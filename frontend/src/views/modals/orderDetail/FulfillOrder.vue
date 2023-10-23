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
      <template v-if="ob.contractType === 'PHYSICAL_GOOD' && !ob.isLocalPickup">
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderShippingCarrier" class="required">{{
              ob.polyT(`orderDetail.fulfillOrderTab.shippingCarrierLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="ob.errors['physicalDelivery.shipper']" :errors="ob.errors['physicalDelivery.shipper']" />
            <input type="text" class="clrBr clrSh2" name="physicalDelivery.shipper" id="fulfillOrderShippingCarrier"
              v-model="info.physicalDelivery.shipper"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.shippingCarrierPlaceholder`)" />
          </div>
        </div>
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderTrackingNumber">{{ ob.polyT(`orderDetail.fulfillOrderTab.trackingLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="ob.errors['physicalDelivery.trackingNumber']"
              :errors="ob.errors['physicalDelivery.trackingNumber']" />
            <input type="text" class="clrBr clrSh2" name="physicalDelivery.trackingNumber" id="fulfillOrderTrackingNumber"
              v-model="info.physicalDelivery.trackingNumber"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.trackingPlaceholder`)" />
          </div>
        </div>
      </template>

      <template v-else-if="ob.contractType === 'DIGITAL_GOOD'">
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderFileUrl" class="required">{{ ob.polyT(`orderDetail.fulfillOrderTab.fileUrlLabel`)
            }}</label>
          </div>
          <div class="col7">
            <FormError v-if="ob.errors['digitalDelivery.url']" :errors="ob.errors['digitalDelivery.url']" />
            <input type="text" class="clrBr clrSh2" name="digitalDelivery.url" id="fulfillOrderFileUrl"
              v-model="info.digitalDelivery.url" :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.fileUrlPlaceholder`)" />
          </div>
        </div>
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderPassword">{{ ob.polyT(`orderDetail.fulfillOrderTab.passwordLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="ob.errors['digitalDelivery.password']" :errors="ob.errors['digitalDelivery.password']" />
            <input type="text" class="clrBr clrSh2" name="digitalDelivery.password" id="fulfillOrderPassword"
              v-model="info.digitalDelivery.password"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.passwordPlaceholder`)" />
          </div>
        </div>
      </template>

      <template v-else-if="ob.contractType === 'CRYPTOCURRENCY'">
        <div class="flexRow gutterH">
          <div class="col3">
            <label for="fulfillOrderTransactionID" class="required">{{
              ob.polyT(`orderDetail.fulfillOrderTab.transactionIDLabel`) }}</label>
          </div>
          <div class="col7">
            <FormError v-if="ob.errors['cryptocurrencyDelivery.transactionID']" :errors="ob.errors['cryptocurrencyDelivery.transactionID']" />
            <input type="text" class="clrBr clrSh2" name="cryptocurrencyDelivery.transactionID"
              id="fulfillOrderTransactionID"
              v-model="info.cryptocurrencyDelivery.transactionID"
              :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.transactionIDPlaceholder`)"
              :maxlength="ob.constraints.transactionIDLength" />
          </div>
        </div>
      </template>
      <div class="flexRow gutterH rowHg">
        <div class="col3">
          <label for="fulfillOrderNote">{{ ob.polyT(`orderDetail.fulfillOrderTab.noteLabel`) }}</label>
        </div>
        <div class="col7">
          <FormError v-if="ob.errors['note']" :errors="ob.errors['note']" />
          <textarea rows="6" name="note" :class="`clrBr clrP clrSh2 ${ob.contractType === 'DIGITAL_GOOD' ? 'rowSm' : ''}`"
            id="fulfillOrderNote" :placeholder="ob.polyT(`orderDetail.fulfillOrderTab.notePlaceholder`)"
            v-model="info.note"></textarea>
          <template v-if="ob.contractType === 'DIGITAL_GOOD'">
            <div class="clrT2 txSm">{{ ob.polyT(`orderDetail.fulfillOrderTab.noteHelperTextDigital`) }}</div>
          </template>
        </div>
      </div>
    </form>
    <hr class="clrBr" />
    <div class="buttonBar flexHRight flexVCent gutterHLg">
      <a class="js-cancel" :disabled="processing" @click="onClickCancel">{{
        ob.polyT(`orderDetail.fulfillOrderTab.btnCancel`) }}</a>
      <ProcessingButton
        :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph js-submit ${processing ? 'processing' : ''}`"
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
  props: {
    options: {
      type: Object,
      default: {},
	  },
  },
  data () {
    return {
      info: {
        physicalDelivery: {
          trackingNumber: '',
        },
        digitalDelivery: {
          password: '',
        },
        cryptocurrencyDelivery: {
          transactionID: '',
        },
        note: '',
      },
      processing: fulfillingOrder(this.orderID),
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();

    this.$children.filter(c => c.$options._componentTag in ['select', 'input', 'textarea'])[0].focus();
    // this.$el.find('select, input, textarea')[0].focus();
  },
  computed: {
    ob () {
      const cryptoDelivery = this.model.get('cryptocurrencyDelivery');

      return {
        ...this.templateHelpers,
        contractType: this.contractType,
        isLocalPickup: this.isLocalPickup,
        ...this._model,
        errors: this.model.validationError || {},
        fulfillingOrder: fulfillingOrder(this.model.id),
        constraints: cryptoDelivery && cryptoDelivery.constraints || {},
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

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

      this.listenTo(orderEvents, 'fulfillingOrder', this.onFulfillingOrder);
      this.listenTo(orderEvents, 'fulfillOrderComplete, fulfillOrderFail', this.onFulfillOrderAlways);
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
    },

    onClickSubmit () {
      this.model.set(this.info);
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
        this.processing = true;
      }
    },

    onFulfillOrderAlways (e) {
      if (e.id === this.model.id) {
        this.processing = false;
      }
    },

    render () {
      this.processing = fulfillingOrder(this.model.id);

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
