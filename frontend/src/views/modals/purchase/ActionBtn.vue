<template>
  <div>
    <div class="posR">
      <div v-if="phase === 'pay' || phase === 'processing'">
        <ProcessingButton :className="'btn width100 clrBAttGrad clrBrDec1 clrTOnEmph js-payBtn ' +
          `${initPay} ${phase} ${outdatedHash ? 'row' : ''}`" :btnText="ob.polyT('purchase.pay')" />

        <div class="txCtr rowSm" v-if="showOutdatedHashErr">
          {{
        ob.purchaseErrT(
          ob.polyT('purchase.errors.outdatedHash', {
            reloadLink: '<a class="js-reloadOutdated">' + `${ob.polyT('purchase.errors.reloadOutdatedHash')}</a>`,
          })
        )
      }}
        </div>
      </div>
      <div v-else-if="phase ==='pending'" class="btn width100 clrBAttGrad clrBrDec1 clrTOnEmph pendingBtn">
        {{ ob.polyT('purchase.pending') }}
      </div>
      <button v-else-if="phase ==='complete'" class="btn width100 clrBAttGrad clrBrDec1 clrTOnEmph js-closeBtn">
        {{ ob.polyT('purchase.close') }}
      </button>

      <div v-if="ob.confirmOpen" id="confirmPay" class="confirmBox arrowBoxCenteredTop clrBr clrP clrT clrSh1 js-confirmPay">
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
          <a class="js-confirmPayCancel">
            {{ ob.polyT('purchase.confirmPayment.cancel') }}
          </a>
          <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph js-confirmPayConfirm">
            {{ ob.polyT('purchase.confirmPayment.confirm') }}
          </a>
        </div>
      </div>
    </div>
    <div class="padSm flexColRows gutterVSm txSm txCtr clrT2">
      <span v-if="phase === 'pay'" class="js-payNote">{{ ob.polyT('purchase.payNote') }}</span>
      <span v-else-if="phase ==='pending'" class="js-pendingNote">{{ ob.polyT('purchase.pendingNote') }}</span>
      <span v-else-if="phase ==='complete'" class="js-closeNote">{{ ob.polyT('purchase.closeNote') }}</span>
      <a class="clrTErr txU" href="https://mobazha.org/scam-prevention">{{ ob.polyT('purchase.scamWarning') }}</a>
    </div>

  </div>
</template>

<script setup>
const props = defineProps({
  phase: String,
  outdatedHash: String,
})

const showOutdatedHashErr = props.phase === 'pay' && props.outdatedHash;
const initPay = (ob.listing.shippingOptions && ob.listing.shippingOptions.length) || showOutdatedHashErr ? 'disabled' : '';
</script>
<style lang="scss" scoped>
</style>
