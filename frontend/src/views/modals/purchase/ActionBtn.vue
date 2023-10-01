<template>
  <div class="actionBtn">
    <div class="posR">
      <template v-if="ob.phase === 'pay' || ob.phase === 'processing'">
        <ProcessingButton
          :className="`btn width100 clrBAttGrad clrBrDec1 clrTOnEmph ${ob.phase} ${outdatedHash ? 'row' : ''}`"
          :disabled="initPay"
          @click="clickPayBtn"
          :btnText="ob.polyT('purchase.pay')" />
        <div v-if="showOutdatedHashErr" class="txCtr rowSm">${ob.purchaseErrT({ tip: errTip })}</div>
      </template>
      <template v-else-if="ob.phase === 'pending'">
        <div class="btn width100 clrBAttGrad clrBrDec1 clrTOnEmph pendingBtn">
          {{ ob.polyT('purchase.pending') }}
        </div>
      </template>

      <template v-else-if="ob.phase === 'complete'">
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
            <a class="" @click="closeConfirmPay">
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
      <template v-if="ob.phase === 'pay'">
        <span class="js-payNote">{{ ob.polyT('purchase.payNote') }}</span>
      </template>

      <template v-else-if="ob.phase === 'pending'">
        <span class="js-pendingNote">{{ ob.polyT('purchase.pendingNote') }}</span>
      </template>

      <template v-else-if="ob.phase === 'complete'">
        <span class="js-closeNote">{{ ob.polyT('purchase.closeNote') }}</span>
      </template>
      <a class="clrTErr txU" href="https://mobazha.org/scam-prevention">{{ ob.polyT('purchase.scamWarning') }}</a>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import loadTemplate from '../../../../backbone/utils/loadTemplate';
import Listing from '../../../../backbone/models/listing/Listing';

export default {
  props: {
  },
  data () {
    return {
      phase: 'pay',
      confirmOpen: false,
      outdatedHash: false,
    };
  },
  mounted () {
    loadData(props);

    render();
  },
  computed: {
    showOutdatedHashErr () {
      return ob.phase === 'pay' && this.outdatedHash;
    },

    initPay () {
      return (options.listing.shippingOptions && options.listing.shippingOptions.length) || showOutdatedHashErr;
    },

    errTip () {
      return ob.polyT('purchase.errors.outdatedHash', {
        eloadLink: '<a class="" @click="clickReloadOutdated" >' + `${ob.polyT('purchase.errors.reloadOutdatedHash')}</a>`,
      });
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.listing || !(options.listing instanceof Listing)) {
        throw new Error('Please provide a listing model.');
      }

      this.options = opts;

      this.boundOnDocClick = this.documentClick.bind(this);
      $(document).on('click', this.boundOnDocClick);
    },

    documentClick (e) {
      if (this.confirmOpen &&
        !($.contains(this.getCachedEl('.js-confirmPay')[0], e.target))) {
        this.confirmOpen = false;
      }
    },

    clickPayBtn (e) {
      e.stopPropagation();
      this.confirmOpen = true;
    },

    clickConfirmBtn () {
      this.$emit('purchase');
    },

    closeConfirmPay () {
      this.confirmOpen = false;
    },

    clickCloseBtn () {
      this.$emit('close');
    },

    clickReloadOutdated () {
      this.$emit('reloadOutdated');
    },

    remove () {
      $(document).off('click', this.boundOnDocClick);
    },

    render () {
      const loadPurchasErrTemplIfNeeded = (tPath, func) => {
        if (this.outdatedHash) return loadTemplate(tPath, func);
        func(null);
        return undefined;
      };

      loadPurchasErrTemplIfNeeded('modals/listingDetail/purchaseError.html', purchaseErrT => {
        loadTemplate('modals/purchase/actionBtn.html', t => {
          this.$el.html(t({
            purchaseErrT,
          }));
        });
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
