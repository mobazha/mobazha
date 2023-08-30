<template>
  <div>

    <div v-if="ob.viewType === 'grid'" class="gridViewContent posR">
      <div class="listingImage js-listingImage" :style="listingImageBgStyle">
        <div class="nsfwOverlay overlayPanel coverFull clrP">
          <div class="flexCent">
            <div>
              <div class="flexCol flexHCent gutterV">
                <div class="flexHCent gutterHSm tx3">
                  {{ ob.parseEmojis('😲') }}
                  {{ ob.parseEmojis('😱') }}
                </div>
                <button class="btn clrP clrBr tx6 clrSh1 js-showNsfw">{{ ob.polyT('listingCard.btnShowMatureContent') }}</button>
              </div>
            </div>
          </div>
        </div>
        <div class="blockedOverlay overlayPanel coverFull clrP">
          <div class="flexCent">
            <div>
              <div class="flexCol flexHCent">
                <i class="ion-eye-disabled tx1"></i>
                <div>{{ ob.polyT('listingCard.blockedUser') }}</div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="ob.ownListing" class="editOverlay overlayPanel">
          <div class="overlayPanelInner"></div>
          <div class="flex gutterHSm">
            <button class='iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-edit' :data-tip="ob.polyT('listingCard.editListingTooltip')"><span class="ion-edit"></span></button>
            <button class='iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-clone' :data-tip="ob.polyT('listingCard.cloneListingTooltip')"><span class="ion-ios-copy"></span></button>
            <div class="posR">
              <a class='iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-delete' :data-tip="ob.polyT('listingCard.deleteListingTooltip')"><span class="ion-trash-b"></span></a>
              <div class="js-deleteConfirmedBox confirmBox deleteConfirm tx5 arrowBoxBottom clrBr clrP clrT hide">
                <div class="tx3 txB rowSm">{{ ob.polyT('listingCard.confirmDelete.title') }}</div>
                <p>{{ ob.polyT('listingCard.confirmDelete.body') }}</p>
                <hr class="clrBr row" />
                <div class="flexHRight flexVCent gutterHLg buttonBar">
                  <a class="js-deleteConfirmCancel">{{ ob.polyT('listingCard.confirmDelete.btnCancel') }}</a>
                  <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph js-deleteConfirmed">{{ ob.polyT('listingCard.confirmDelete.btnConfirm') }}</a>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="additionalOverlay overlayPanel">
          <div class="flex gutterHSm">
            <div class="hideIfEmpty js-reportBtnWrapper"></div>
            <div class="js-blockBtnWrapper"></div>

            <button v-if="ob.nsfw" class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop btnHideNsfw js-hideNsfw" :data-tip="ob.polyT('listingCard.tipHideMatureContent')"><i class="ion-locked"></i></button>
          </div>
        </div>

      </div>
      <div class="pad clrBr borderTop infoArea">

        <div v-if="ob.vendor">
          <a class="userIconWrapper js-userLink" :href="`#${ob.vendor.peerID}/store`">
            <div class="userIcon disc clrBr2 clrSh1 toolTipNoWrap js-vendorIcon" :style="`background-image: ${vendorAvatarImageSrc}url('../imgs/defaultAvatar.png')`" :data-tip="ob.vendor.name">
            </div>
          </a>
          <div class="userIconWrapper nsfwAvatarOverlay">
            <div class="userIcon disc clrBr2 clrSh1 clrP tx3">
              {{ ob.parseEmojis('😲') }}
            </div>
          </div>
          <div class="userIconWrapper blockedAvatarOverlay">
            <div class="userIcon disc clrBr2 clrSh1 clrP">
              <i class="ion-eye-disabled"></i>
            </div>
          </div>
        </div>

        <div class="rowSm">
          <!-- // The accepted currencies check is in case that data is not provided (e.g. a search provider omits),
      // we'll fall back to displaying the title field, since otherwise we're unable to determine one of
      // the trading pairs. -->
          <div v-if="ob.contractType !== 'CRYPTOCURRENCY' || (!(ob.acceptedCurrencies && ob.acceptedCurrencies.length))"
            :class="`${ob.title.length > 60 ? 'toolTip' : 'toolTipNoWrap'} toolTipTop inlineBlock ${ob.vendor ? 'trimWidth' : ''}`" :data-tip="ob.title">
            <a class="clrT clamp listingTitle">{{ ob.title }}</a>
          </div>
          <div v-else>
            <a class="listingTitle">
              {{ ob.crypto.tradingPair({
            className: 'cryptoTradingPairSm',
            fromCur: `${ob.acceptedCurrencies && ob.acceptedCurrencies[0]}`,
            toCur: ob.price.currencyCode,
            truncateCurAfter: 5,
          }) }}
            </a>
          </div>
        </div>

        <div :class="`flexVCent ${priceRowTextClass}`">
          <div class="flexExpand ratingStrip">
            {{ formattedRating }}
          </div>

          <div v-if="ob.contractType !== 'CRYPTOCURRENCY'">
            {{
          ob.currencyMod.convertAndFormatCurrency(
            ob.price.amount,
            ob.price.currencyCode,
            ob.displayCurrency,
            { maxDisplayDecimals: priceMaxDisplayDecimals }
          )
        }}
          </div>
          <div v-else>
            {{
              ob.crypto.cryptoPrice({
                priceAmount: ob.price.amount,
                priceCurrencyCode: ob.price.currencyCode,
                displayCurrency: ob.displayCurrency,
                priceModifier: ob.price.modifier,
                wrappingClass: '',
                marketRelativityClass: 'hide',
                convertAndFormatOpts: {
                  maxDisplayDecimals: priceMaxDisplayDecimals
                }
              })
            }}
          </div>

        </div>
      </div>
      <div class="listingIcons">
        <span v-if="ob.shipsFreeToMe" class="clrE1 clrTOnEmph phraseBox">{{ ob.polyT('listingCard.freeShippingBanner') }}</span>
      </div>
      <div class="verifiedModWrapper js-verifiedMod"></div>
    </div>

    <div v-else-if="ob.viewType === 'list'" class="listViewContent">
      <div class="flexVCent gutterHSm">
        <!-- // Since we have inconsistent padding/gutters, we'll inline some padding settings. -->
        <div class="flexNoShrink posR">
          <div class="listingImage js-listingImage posR" :style="listingImageBgStyle"></div>
          <div class="center tx2 nsfwAvatarOverlay">{{ ob.parseEmojis('😲') }}</div>
        </div>
        <div class="flexExpand">

          <div v-if="ob.contractType !== 'CRYPTOCURRENCY' ||
            !Array.isArray(ob.acceptedCurrencies) ||
            !ob.acceptedCurrencies.length">
            <div :class="`rowTn inlineBlock ${ob.title.length > 60 ? 'toolTip' : 'toolTipNoWrap'} toolTipTop`" :data-tip="ob.title">
              <a class="clrT clamp3 listingTitle">{{ ob.title }}</a>
            </div>
          </div>
          <div v-else>
            {{ ob.crypto.tradingPair({
            className: 'cryptoTradingPairSm',
            fromCur: ob.acceptedCurrencies[0],
            toCur: ob.price.currencyCode,
            truncateCurAfter: 5,
          }) }}
          </div>
          <div class="flexVCent gutterHSm tx5b">
            <div class="flexNoShrink ratingStrip">
              {{ ob.formatRating(ob.averageRating, ob.ratingCount) }}
            </div>
            <div class="verifiedModWrapper js-verifiedMod"></div>
          </div>
        </div>

        <div v-if="ob.shipsFreeToMe" class="freeShipCol flexNoShrink txCtr">
          <span class="clrE1 clrTOnEmph clamp4 phraseBox">{{ ob.polyT('listingCard.freeShippingBanner') }}</span>
        </div>

        <div class="priceCol flexNoShrink clamp4">
          <span>
            {{
              ob.contractType !== 'CRYPTOCURRENCY'
                ? ob.currencyMod.convertAndFormatCurrency(
                  ob.price.amount,
                  ob.price.currencyCode,
                  ob.displayCurrency,
                  { maxDisplayDecimals: priceMaxDisplayDecimals }
                )
                : ob.crypto.cryptoPrice({
                  priceAmount: ob.price.amount,
                  priceCurrencyCode: ob.price.currencyCode,
                  displayCurrency: ob.displayCurrency,
                  priceModifier: ob.price.modifier,
                  wrappingClass: '',
                  convertAndFormatOpts: {
                    maxDisplayDecimals: priceMaxDisplayDecimals
                  }                
                })
            }}
          </span>
        </div>
      </div>
    </div>

    <div v-else class="flexVCent gutterH">
      <div class="tradeFromCol">
        <div class="flexVCent gutterHSm">
          {{ ob.polyT('cryptoCodeIconPairing', {
              code: `<span class="txB clamp">${ob.acceptedCurrencies && ob.acceptedCurrencies[0] || ''}</span>`,
              icon: ob.crypto.cryptoIcon({
                code: `${ob.acceptedCurrencies && ob.acceptedCurrencies[0] || ''}`,
                className: 'flexNoShrink',
              }),
        }) }}
        </div>
      </div>
      <div class="tradeArrowCol">
        <span class="pairingSeparator clrT2 ion-android-arrow-forward"></span>
      </div>
      <div class="tradeToCol">
        <div class="flexVCent gutterHSm">
          {{ 
          ob.polyT('cryptoCodeIconPairing', {
              code: `<span class="txB clamp">${ob.coinType}</span>`,
              icon: ob.crypto.cryptoIcon({ code: ob.coinType }),
          })
        }}
        </div>
      </div>
      <div class="vendorCol">
        <div class="flex gutterH">
          <div>
            <a class="userIcon disc clrBr2 clrSh1 js-userLink" :style="ob.getAvatarBgImage(ob.vendor.avatarHashes)" :href="`#${ob.vendor.peerID}/store`"></a>
          </div>
          <div class="flexCol gutterVTn">
            <div>
              <a class="clrT clamp js-userLink" :href="`#${ob.vendor.peerID}/store`">{{ ob.vendor.name }}</a>
            </div>

            <div class="cl2amp clrT2 tx6 flexVCent gutterHSm">
              <div class="ratingStrip">
                {{ ob.vendor && ob.vendor.stats ? ob.formatRating(averageStoreRating, totalStoreRatings) : ob.formatRating(0, 0) }}
              </div>
              <div class="verifiedModWrapper">
                <div class="js-verifiedMod"></div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="priceCol">
        {{
          ob.crypto.cryptoPrice({
            priceAmount: ob.price.amount,
            priceCurrencyCode: ob.price.currencyCode,
            displayCurrency: ob.displayCurrency,
            priceModifier: ob.price.modifier,
            wrappingClass: 'txB',
          })
        }}
      </div>

      <!-- // This is being commented out until inventory is functional.
    <div class="inventoryCol flexExpand flexVCent flexHRight gutterHSm {{ inventoryTxClass }}">{{ inventory }}</div> -->

    </div>

    <div v-if="['list', 'cryptoList'].includes(ob.viewType)">
      <div class="blockedOverlay overlayPanel coverFull clrP">
        <div class="flexCent">
          <div>
            <div class="flexCol flexHCent">
              <i class="ion-eye-disabled tx4"></i>
              <div>{{ ob.polyT('listingCard.blockedUser') }}</div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="ob.ownListing" class="editOverlay overlayPanel clrP">
        <div class="overlayPanelInner"></div>
        <div class="flexHCent gutterHSm">
          <button class='iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-edit' :data-tip="ob.polyT('listingCard.editListingTooltip')"><span class="ion-edit"></span></button>
          <button class='iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-clone' :data-tip="ob.polyT('listingCard.cloneListingTooltip')"><span class="ion-ios-copy"></span></button>
          <div class="posR">
            <button class='iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-delete' :data-tip="ob.polyT('listingCard.deleteListingTooltip')"><span class="ion-trash-b"></span></button>
            <div class="js-deleteConfirmedBox confirmBox deleteConfirm tx5 arrowBoxBottom clrBr clrP clrT hide">
              <div class="tx3 txB rowSm">{{ ob.polyT('listingCard.confirmDelete.title') }}</div>
              <p>{{ ob.polyT('listingCard.confirmDelete.body') }}</p>
              <hr class="clrBr row" />
              <div class="flexHRight flexVCent gutterHLg buttonBar">
                <a class="js-deleteConfirmCancel">{{ ob.polyT('listingCard.confirmDelete.btnCancel') }}</a>
                <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph js-deleteConfirmed">{{ ob.polyT('listingCard.confirmDelete.btnConfirm') }}</a>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="additionalOverlay overlayPanel clrP">
        <div class="flex gutterHSm">
          <div class="hideIfEmpty js-reportBtnWrapper"></div>
          <div class="js-blockBtnWrapper"></div>
          <button class="btn clrP clrBr iconBtnSm btnShowNsfw clrSh1 js-showNsfw toolTipNoWrap toolTipTop" tabindex="0" :data-tip="ob.polyT('listingCard.tipShowMatureContent')"><i class="ion-unlocked"></i></button>

          <button v-if="ob.nsfw" class="btn clrP clrBr iconBtnSm btnHideNsfw clrSh1 js-hideNsfw toolTipNoWrap toolTipTop" tabindex="0" :data-tip="ob.polyT('listingCard.tipHideMatureContent')"><i class="ion-locked"></i></button>
        </div>
      </div>
    </div>

    <div class="deleteOverlay coverFull overlayPanel">
      <div class="overlayPanelInner clrS"></div>
      <div class="deletingText clrT tx5">{{ ob.polyT('listingCard.deleting') }}</div>
      <div class="deletedText clrT tx5">
        <div class="ion-trash-b tx3"></div>
        {{ ob.polyT('listingCard.deleted') }}
      </div>
    </div>

  </div>
</template>

<script setup>
const props = defineProps({
  phase: String,
})

const listingImageSrc = ob.listingImageSrc ? `url(${ob.listingImageSrc}), ` : '';
const listingImageBgStyle = `background-image: ${listingImageSrc}url('../imgs/defaultItem.png')`;

const vendorAvatarImageSrc = ob.vendorAvatarImageSrc ? `url(${ob.vendorAvatarImageSrc}), ` : '';

let priceMaxDisplayDecimals;

try {
  if (!ob.currencyMod.isFiatCur(ob.displayCurrency)) {
    priceMaxDisplayDecimals = 6;
  }
} catch (e) {
  // pass
}

const formattedRating = ob.formatRating(ob.averageRating, ob.ratingCount);
let priceRowTextClass = '';

try {
  const priceLength = ob.price.amount.toFormat().length;
  const ratingLength = ($(`<div>${formattedRating}</div>`).text()).length;

  if (priceLength + ratingLength > 17) {
    priceRowTextClass = 'txBase'
  }
} catch (e) {
  // pass
}

let inventory = ob.polyT('cryptoAmountIconPairing', {
  amount: `<i class="clrT2">${ob.polyT('inventoryDisplay.unknownInventory')}</i>`,
  icon: ob.crypto.cryptoIcon({ code: ob.coinType }),
});


if (ob.totalInventoryQuantity >= 0) {
  let formattedAmount = ob.currencyMod.formatCurrency(ob.totalInventoryQuantity, ob.coinType,
    { includeCryptoCurIdentifier: false, maxDisplayDecimals: 4 });

  if (formattedAmount.length >= 18) {
    formattedAmount = ob.abbrNum(ob.totalInventoryQuantity, 2);
  }

  inventory = ob.polyT('cryptoAmountIconPairing', {
    amount: `<span>${formattedAmount}</span>`,
    icon: ob.crypto.cryptoIcon({ code: ob.coinType }),
  });
}

let inventoryTxClass = '';
if (inventory.length > 14) {
  inventoryTxClass = 'tx6';
} else if (inventory.length > 10) {
  inventoryTxClass = 'tx5b';
}

</script>
<style lang="scss" scoped>
</style>