<template>
  <div class="actionBtn" @click="documentClick">
    <div class="posR">
      <template v-if="phase === 'pay' || phase === 'processing'">
        <ProcessingButton
          :className="`btn width100 clrBAttGrad clrBrDec1 clrTOnEmph ${phase} ${outdatedHash ? 'row' : ''}`"
          :disabled="initPay"
          @click.stop="clickPayBtn"
          :btnText="ob.polyT('purchase.pay')" />
        <div v-if="showOutdatedHashErr" class="txCtr rowSm">
          <PurchaseError :tip="errTip" />
        </div>
      </template>
      <template v-else-if="phase === 'pending'">
        <div class="btn width100 clrBAttGrad clrBrDec1 clrTOnEmph pendingBtn">
          {{ ob.polyT('purchase.pending') }}
        </div>
      </template>

      <template v-else-if="phase === 'complete'">
        <button class="btn width100 clrBAttGrad clrBrDec1 clrTOnEmph " @click="clickCloseBtn">
          {{ ob.polyT('purchase.close') }}
        </button>
      </template>
      <template v-if="confirmOpen">
        <div id="confirmPay" class="confirmBox arrowBoxCenteredTop clrBr clrP clrT clrSh1 js-confirmPay">
          <div class="flexColRows gutterVSm padLg">
            <h3>
              {{ ob.polyT('purchase.confirmPayment.title') }}
            </h3>
            <p class="tx5">
              {{ ob.polyT('purchase.confirmPayment.msg') }}
            </p>
          </div>
          <hr class="unleaded clrBr" />
          <div class="flexHRight flexVCent gutterHLg pad tx5">
            <a class="" @click.stop="closeConfirmPay">
              {{ ob.polyT('purchase.confirmPayment.cancel') }}
            </a>
            <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click="clickConfirmBtn">
              {{ ob.polyT('purchase.confirmPayment.confirm') }}
            </a>
          </div>
        </div>
      </template>
    </div>
    <div class="padSm flexColRows gutterVSm txSm txCtr clrT2">
      <template v-if="phase === 'pay'">
        <span class="js-payNote">{{ ob.polyT('purchase.payNote') }}</span>
      </template>

      <template v-else-if="phase === 'pending'">
        <span class="js-pendingNote">{{ ob.polyT('purchase.pendingNote') }}</span>
      </template>

      <template v-else-if="phase === 'complete'">
        <span class="js-closeNote">{{ ob.polyT('purchase.closeNote') }}</span>
      </template>
      <a class="clrTErr txU" href="https://mobazha.org/scam-prevention">{{ ob.polyT('purchase.scamWarning') }}</a>
    </div>

  </div>
</template>

<script>
import Listing from '../../../../backbone/models/listing/Listing';

import PurchaseError from '@/views/modals/listingDetail/PurchaseError.vue'

export default {
  components: {
    PurchaseError,
  },
  props: {
    phase: {
      type: String,
      default: 'pay',
	  },
    outdatedHash: {
      type: Boolean,
      default: false,
	  },
    
    bb: Function,
  },
  data () {
    return {
      confirmOpen: false,
    };
  },
  created () {
    this.loadData();
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
      };
    },
    showOutdatedHashErr () {
      return this.phase === 'pay' && this.outdatedHash;
    },

    initPay () {
      return (this.oneListing.shippingOptions && this.oneListing.shippingOptions.length) || this.showOutdatedHashErr;
    },

    errTip () {
      const ob = this.ob;
      return ob.polyT('purchase.errors.outdatedHash', {
        reloadLink: '<a class="" @click="clickReloadOutdated" >' + `${ob.polyT('purchase.errors.reloadOutdatedHash')}</a>`,
      });
    }
  },
  methods: {
    loadData () {
      if (!this.oneListing || !(this.oneListing instanceof Listing)) {
        throw new Error('Please provide a listing model.');
      }
    },

    documentClick (e) {
      if (this.confirmOpen) {
        this.confirmOpen = false;;
      }
    },

    clickPayBtn () {
      this.confirmOpen = true;;
    },

    clickConfirmBtn () {
      this.$emit('purchase');
    },

    closeConfirmPay () {
      this.confirmOpen = false;;
    },

    clickCloseBtn () {
      this.$emit('close');
    },

    clickReloadOutdated () {
      this.$emit('reloadOutdated');
    },
  }
}
</script>
<style lang="scss" scoped></style>
