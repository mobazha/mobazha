<template>
  <div>
    <!--
  This view is used by both the Purchase and Order detail flows. When making changes, please ensure they play nice with both areas.
-->

    <div :class="`flexRow gutterHLg ${ob.externallyFundable ? `pad rowMd` : ''}`">
      <div class="flexNoShrink" v-if="ob.externallyFundable">
        <a class="QR js-purchaseQRCode"><img class="js-qrCodeImg" :src="ob.qrDataUri"></a>
      </div>

      <div class="flexExpand">
        <div class="flexVCent">
          <div class="flexExpand">
            <div class="rowSm clickable js-amountRow">
              <span class="h1 js-amountDueLine">{{ ob.amountDueLine }}</span>
              <button class="btnTxtOnly txUnb flipBtn js-copyAmount" v-if="ob.externallyFundable">
                <span class="clrTEm unFlipped">{{ ob.polyT('purchase.pendingSection.copy') }}</span>
                <span class="flipped">{{ ob.polyT('purchase.pendingSection.copied') }}</span>
              </button>
            </div>

            <div class="tx5 rowMd clickable js-addressRow" v-if="ob.externallyFundable">
              <span
                :class="ob.paymentAddress.length > 34 ? 'toolTipNoWrap toolTipTop' : ''"
                :data-tip="ob.paymentAddress.length > 34 ? ob.paymentAddress : ''">
                {{ ob.polyT('purchase.pendingSection.to', { address: pAddress }) }}
              </span>
              <button class="btnTxtOnly txUnb flipBtn js-copyAddress">
                <span class="clrTEm unFlipped">{{ ob.polyT('purchase.pendingSection.copy') }}</span>
                <span class="flipped">{{ ob.polyT('purchase.pendingSection.copied') }}</span>
              </button>
            </div>
            <div :class="`flexRow gutterH ${ob.externallyFundable ? 'rowLg' : 'rowMd'}`">
              <div class="col6">
                <%= ob.processingButton({
            className: 'btn btnThin width100 clrP clrBr clrSh2 js-payFromWallet',
            textClassName: 'flexCent',
            btnText: `<i class="icon">${ob.walletIconTmpl()}</i>${ob.polyT('purchase.pendingSection.payFromWallet')}`,
            }) %>
                <div class="js-confirmWalletContainer"></div>
              </div>
            </div>

            <div class="txBase clrT2" v-if="ob.externallyFundable">
              <p v-if="['BTC', 'TBTC'].includes(ob.paymentCoin)">
                {{ ob.polyT('purchase.pendingSection.walletNote') }} <button class="btnAsLink js-fundWallet">{{ ob.polyT('purchase.pendingSection.walletLink') }}</button>
              </p>
              <p>
                {{ ob.polyT('purchase.pendingSection.feeNote') }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="tx6 clrT2">
      <p v-if="['BTC', 'TBTC'].includes(ob.paymentCoin)">
      {{ 
        ob.polyT('purchase.pendingSection.note1', {
          link: `<a class="clrTEm" href="https://www.openbazaar.org/bitcoin">${ob.polyT('purchase.pendingSection.note1Link')}</a>`,
        })
      }}</p>

      <p v-if="ob.isModerated">
        {{ ob.polyT('purchase.pendingSection.note2') }}
        <br>
        {{ ob.polyT('purchase.pendingSection.note3') }}
      </p>
      <p>{{ ob.polyT('purchase.pendingSection.note4') }}</p>
    </div>

  </div>
</template>

<script setup>
const pAddress = ob.paymentAddress.length > 34 ? `${ob.paymentAddress.slice(0, 34)}…` : ob.paymentAddress;
</script>
<style lang="scss" scoped>
</style>