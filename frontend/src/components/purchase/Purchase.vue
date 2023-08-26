<template>
  <div class="modal purchase modalScrollPage">
    <div class="popInMessageHolder js-popInMessages"></div>

    <div class="topControls gutterHSm flex">
      <div v-if="ob.vendor" class="contentBox clrP clrSh3 clrBr clrT">
        <div class="padSm gutterHSm overflowAuto margRSm flexVCent">
          <a class="clrBr2 clrSh1 discTn flexNoShrink" style="{{ ob.getAvatarBgImage(ob.vendor.avatarHashes) }}"></a>
          <p class="txUnl tx3 clamp">{{ ob.vendor.name }}</p>
          <a class="link flexNoShrink tx6 js-goToListing">{{ ob.polyT('purchase.returnToListing') }}</a>
        </div>
      </div>
    </div>

    <div :class="`flexRow gutterH mainSection ${ob.phaseClass}`">
      <div class="col9">
        <div class="flexColRow gutterV">
          <section class="contentBox pad clrP clrBr clrSh3" v-for="(listing, key) in ob.listings" :key="key">
            <div class="js-errors"></div>
            <div class="js-items-quantity-errors"></div>

            <div class="flexVCent gutterH" v-if="!ob.isCrypto">
              <div class="thumb" :style="ob.getListingBgImage(listing.listing.item.images[0])"></div>
              <div class="flexExpand">
                <div class="flexCol gutterVTn">
                  <div class="width100 noOverflow">
                    <b>{{ listing.listing.item.title }}</b>
                  </div>

                  <div class="width100 noOverflow" v-for="(variant, i) in listing.options" :key="i">
                    <span class="clrT2">{{ variant.name }}: {{ variant.value }}</span>
                  </div>

                </div>
              </div>

              <div class="flexNoShrink" v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                <div class="flexVCent gutterH">
                  <div class="flexCol">
                    <label class="flexHRight" for="purchaseQuantity">
                      <span class="required txB margR">{{ ob.polyT('purchase.quantity') }}</span>
                    </label>
                  </div>
                  <div class="flexNoShrink">
                    <input
                      class="clrBr clrP"
                      type="text"
                      id="purchaseQuantity"
                      size="3"
                      name="quantity"
                      :value="listing.quantity"
                      placeholder="0"
                      data-var-type="bignumber">
                  </div>
                </div>
              </div>

              <div class="pad flexNoShrink"><b>{{ listing.price }}</b></div>
            </div>
            <div v-else>
              <div class="flexVCent gutterHLg row cryptoTitleWrap">
                <div class="js-cryptoTitle <% if (ob.phase !== 'pay' && ob.phase !== 'processing') print('flexExpand') %>"></div>

                <div class="flexExpand" v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                  <div class="flexVCent gutterHLg">
                    <label for="cryptoAmount" class="clrT txB required">{{ ob.polyT('purchase.cryptoAmount') }}</label>
                    <div class="inputSelect">
                      <input type="text" class="clrBr clrP clrSh2" name="quantity" id="cryptoAmount" :value="ob.quantity" placeholder="0.0000" size="8" data-var-type="bignumber">

                      <select id="cryptoAmountCurrency" class="clrBr clrP nestInputRight" v-if="ob.displayCurrency !== listing.listing.item.cryptoListingCurrencyCode">
                        <option v-for="(cur, j) in [listing.listing.item.cryptoListingCurrencyCode, ob.displayCurrency]" :key="j" :value="cur" :selected="cur === ob.cryptoAmountCurrency">{{ cur }}</option>
                      </select>
                    </div>
                  </div>
                </div>

                <div class="pad flexNoShrink">
                  {{ 
                    ob.crypto.cryptoPrice({
                      priceAmount: totalPrice,
                      priceCurrencyCode: pricingCurrency,
                      displayCurrency: ob.displayCurrency,
                      priceModifier: listing.listing.item.cryptoListingPriceModifier,
                    })
                  }}
                </div>
              </div>
              <hr class="clrBr rowLg" />

              <div class="rowSm">
                <label class="h4 flexExpand required" for="purchaseCryptoAddress">{{ ob.polyT('purchase.cryptoAddressHeading', {coinType: coinName}) }}</label>
              </div>
              <div class="js-items-paymentAddress-errors"></div>
              <div v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                <input type="text" id="purchaseCryptoAddress" :value="ob.items[0].paymentAddress" :placeholder="ob.polyT('purchase.cryptoAddressPlaceholder', { coinType: coinName})" class="clrBr clrP rowSm" :maxlength="ob.itemConstraints.maxPaymentAddressLength" />
              </div>
              <p v-else class="cryptoPaymentAddress">{{ ob.items[0].paymentAddress }}</p>

              <div class="txSm clrT2">{{ helper }}</div>
            </div>
          </section>

          <div v-if="ob.phase === 'pay' || ob.phase === 'processing'">
            <section v-if="ob.listing.shippingOptions && ob.listing.shippingOptions.length" class="contentBox padMd clrP clrBr clrSh3 js-shipping">
              <div class="js-shipping-errors js-items-shipping-errors"></div>
              <div class="js-shippingWrapper"></div>
            </section>

            <section class="contentBox padMd clrP clrBr clrSh3">
              <div class="flexColRows gutterVSm">
                <div>
                  <div class="js-paymentCoin-errors"></div>
                  <h2 class="h4 flexExpand required">{{ ob.polyT('purchase.cryptoCurrencyTitle') }}</h2>
                  <div class="js-cryptoCurSelectorWrapper"></div>
                </div>
              </div>
            </section>
            <section class="contentBox padMd clrP clrBr clrSh3">
              <div class="flexColRows gutterVSm">
                <div class="flexVCentClearMarg">
                  <h2 class="h4 flexExpand required">{{ ob.polyT('purchase.paymentTypeTitle') }}</h2>
                  <div v-if="ob.showModerators">
                    <input type="checkbox" id="purchaseVerifiedOnly" class="js-purchaseVerifiedOnly" :checked="ob.showVerifiedOnly">
                    <label class="tx5b" for="purchaseVerifiedOnly">{{ ob.polyT('settings.storeTab.verifiedOnly') }}</label>
                  </div>
                </div>
                <div v-if="ob.showModerators && !ob.noValidModerators">
                  <div class="js-moderated-errors"></div>
                  <div class="js-moderatorsWrapper"></div>
                  <div v-if="!ob.noValidModerators">
                    <div class="clrT2 tx6 rowMd">{{ ob.polyT('purchase.moderatorsDisclaimer') }}</div>
                  </div>
                  <hr class="clrBr row">
                </div>
                <div class="js-directPaymentWrapper moderatorsList"></div>
              </div>
            </section>
            <section class="contentBox padMd clrP clrBr clrSh3">
              <h2 class="h4">
                {{ ob.polyT('purchase.informationTitle') }}
                <span class="clrT2 txUnb tx5b">{{ ob.polyT('purchase.optional') }}</span>
              </h2>
              <div class="flexRow gutterH row">
                <div class="col6">
                  <div class="rowTn">
                    <label for="emailAddress" class="tx5">
                      {{ ob.polyT('purchase.emailAddress') }}
                    </label>
                  </div>
                  <div>
                    <input
                      class="btnHeight clrBr clrP js-purchaseField"
                      type="text"
                      id="emailAddress"
                      name="alternateContactInfo"
                      :placeholder="ob.polyT('purchase.emailPlaceholder')"
                      :value="ob.alternateContactInfo">
                  </div>
                  <div>
                    <span class="txSm clrT2">{{ ob.polyT('purchase.emailNote') }}</span>
                  </div>
                </div>
                <div class="col6">
                  <div v-if="ob.hasCoupons">
                    <div class="rowTn">
                      <label for="couponCode" class="tx5">
                        {{ ob.polyT('purchase.couponCode') }}
                      </label>
                    </div>
                    <div class="flex gutterH row">
                      <input
                        class="btnHeight clrBr clrP"
                        type="text"
                        id="couponCode"
                        name="couponCode"
                        :placeholder="ob.polyT('purchase.couponCodePlaceholder')">
                      <button class="btn clrP clrBr clrSh2 flexNoShrink js-applyCoupon">
                        {{ ob.polyT('purchase.applyCode') }}
                      </button>
                    </div>
                    <div class="js-couponsWrapper">
                      <!-- // coupons are inserted here after they are added by the user. -->
                    </div>
                  </div>
                </div>
              </div>
              <hr class="clrBr row">
              <div class="rowTn">
                <label for="memo" class="tx5">
                  {{ ob.polyT('purchase.memo') }}
                </label>
              </div>
              <textarea
                class="clrBr clrP js-purchaseField"
                id="memo"
                name="memo"
                maxlength="5000"
                rows="6"
                v-model="ob.items[0].memo"
                :placeholder="ob.polyT('purchase.memoPlaceholder')" />
            </section>
          </div>

          <section v-if="ob.phase === 'pending'" class="contentBox padMd clrP clrBr clrSh3 js-pending"></section>

          <section v-if="ob.phase === 'complete'" class="contentBox padMd clrP clrBr clrSh3 js-complete"></section>
        </div>
      </div>
      <div class="col3">
        <section class="contentBox pad clrP clrBr clrSh3 sidebar">
          <i class="js-close cornerTR ion-ios-close-empty iconBtn clrP clrBr clrSh3 closeBtn"></i>
          <div class="js-actionBtn"></div>
          <div class="rowLg">
            <div class="js-receipt"></div>
            <div v-if="ob.showModerators && !ob.noValidModerators">
              <hr class="clrBr">
              <div class="padSm txSm txCtr clrT2">
                {{ ob.polyT('purchase.moderatorNote') }}
              </div>
            </div>
          </div>
          <div class="tx6 js-feeChangeContainer"></div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup>
// when multiple listings are supported, the prices array will have one price object for each
const totalPrice =
  ob.prices[0]
    .price
    .plus(ob.prices[0].vPrice);

const pricingCurrency = ob.listingPrice.currencyCode;


const coinType = listing.listing.item.cryptoListingCurrencyCode;
const coinTranslationKey = `cryptoCurrencies.${coinType}`;
const coinName = ob.polyT(coinTranslationKey) === coinTranslationKey ? coinType : ob.polyT(coinTranslationKey);

const warning = ob.phase === 'pay' || ob.phase === 'processing' ?
  `<b>${ob.polyT('purchase.cryptoAddressHelperWarning')}</b>` :
  `<b>${ob.polyT('purchase.cryptoAddressHelperWarning2')}</b>`;
const helper = ob.polyT('purchase.cryptoAddressHelper', {
  name: ob.vendor.name,
  coinType: coinName,
  warning,
});
</script>
<style lang="scss" scoped>
</style>