<template>
  <div class="contractTab">
    <div class="padLg flexVCent">
      <div class="backToSummaryWrap">
        <a class=" clrTEm txU" @click="onClickBackToSummary">{{ ob.polyT(`orderDetail.backToSummary`) }}</a>
      </div>
      <div class="txCtr txB tx3 flexExpand">{{ ob.polyT(`orderDetail.contractTab.heading`) }}</div>
    </div>
    <hr class="clrBr rowLg" />
    <div class="js-statusContainer rowLg"></div>
    <template v-for="(contract, key) in contracts" :key="key">
      <Contract :options="model.isCase ? contractOptions(contract) : { contract }"/>
    </template>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../../backbone/app';
import Contract from './Contract.vue';


export default {
  components: {
    Contract,
  },
  props: {
    model: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      contracts: [],
    };
  },
  created () {
    this.loadData();
  },
  mounted () {
    this.loadContracts();
  },
  computed: {
  },
  methods: {
    loadData () {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      if (this.model.isCase &&
        (!this.model.get('vendorContract') ||
          !this.model.get('buyerContract'))) {
        this.listenTo(this.model, 'otherContractArrived', (md, data) => {
          const rawContract = this.model.get(`raw${data.isBuyer ? 'Buyer' : 'Vendor'}Contract`);

          if (!this.model.bothContractsValid) this.renderContract(rawContract);
          this.renderStatus();

          if (this.model.bothContractsValid) {
            this[`${data.isBuyer ? 'vendor' : 'buyer'}ContractVw`].setState({ heading: '' });
          }
        });
      }
    },

    onClickBackToSummary () {
      this.$emit('clickBackToSummary');
    },

    renderStatus () {
      const iconBaseClass = 'margRSm flexNoShrink';
      let msg = '';

      if (this.model.isCase) {
        // Cut a corner with some html embedded here. If the html get more elaborate than this,
        // we should probably break this out into its own template.
        if (this.model.bothContractsValid) {
          const icon = `<span class="${iconBaseClass} tx1 ion-ios-checkmark-outline"></span>`;
          const msgText = !this.model.vendorProcessingError ?
            app.polyglot.t('orderDetail.contractTab.bothContractsValid') :
            app.polyglot.t('orderDetail.contractTab.validBuyerVendorProcessingError');
          const msgHtml = `<span>${msgText}</span>`;
          msg = `<p class="clrTEm flexVCent">${icon}${msgHtml}</p>`;
        } else if (!this.model.get('vendorContract')) {
          const icon = `<span class="${iconBaseClass} clrTAlert ion-android-warning"></span>`;
          const processingErrorKey =
            'orderDetail.contractTab.vendorContractNotArrivedPotentialProcErr';
          const buyerContract = this.model.get('buyerContract');
          const buyerShowsVendorProcErr =
            buyerContract && Array.isArray(buyerContract.get('errors'));
          const msgText = !buyerShowsVendorProcErr ?
            app.polyglot.t('orderDetail.contractTab.vendorContractNotArrived') :
            app.polyglot.t(processingErrorKey);
          const msgHtml = `<span>${msgText}</span>`;
          msg = `<p class="flexVCent">${icon}${msgHtml}</p>`;
        } else if (!this.model.get('buyerContract')) {
          msg = `<p class="flexVCent"><span class="${iconBaseClass} clrTAlert ion-android-warning">` +
            `</span>${app.polyglot.t('orderDetail.contractTab.buyerContractNotArrived')}</p>`;
        }
      }

      $('.js-statusContainer').html(msg);
    },

    contractOptions (contract) {
      if (!contract) {
        throw new Error('Please provide a contract.');
      }

      const isBuyerContract = contract === this.model.get('rawBuyerContract');
      let heading = '';

      if (!this.model.bothContractsValid) {
        heading = isBuyerContract ?
          app.polyglot.t('orderDetail.contractTab.contractHeadingBuyer') :
          app.polyglot.t('orderDetail.contractTab.contractHeadingVendor');
      }
      
      return {
        contract,
        initialState: {
          heading,
          errors: isBuyerContract ?
            this.model.get('buyerContractValidationErrors') || [] :
            this.model.get('vendorContractValidationErrors') || [],
        },
      };
    },

    loadContracts () {
      this.renderStatus();

      if (!this.model.isCase) {
        this.contracts.push(this.model.get('rawContract'));
      } else {
        this.contracts = [
          this.model.get('buyerOpened') ?
            this.model.get('rawBuyerContract') :
            this.model.get('rawVendorContract'),
        ];

        if (!this.model.bothContractsValid) {
          // If the second contract has arrived, we'll show them individually since one or
          // both have validation errors.
          if (this.model.get('buyerOpened')) {
            if (this.model.get('vendorContract')) {
              this.contracts.push(this.model.get('rawVendorContract'));
            }
          } else if (this.model.get('buyerContract')) {
            this.contracts.push(this.model.get('rawBuyerContract'));
          }
        }
      }
    }
  }
}
</script>
<style lang="scss" scoped></style>
