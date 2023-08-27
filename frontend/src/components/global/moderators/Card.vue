<template>

  <div :class="`moderatorCardInner clrP ${isDisabled} ${style}`">
    <div class="flexRow gutterH moderatorCardContent">

      <div class="flexNoShrink" v-if="ob.radioStyle">
        <div class="btnRadio">
          <!-- // the card state may be set on render or set on the fly by the view -->
          <div tabindex="0" :class="`fauxRadioBtn js-selectBtn ${ob.selectedState === 'selected' ? 'active' : 'inactive'}`" :data-state="ob.selectedState">
          </div>
        </div>
      </div>

      <div class="flexNoShrink">
        <a class="userIcon disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(ob.avatarHashes)"></a>
      </div>
      <div class="moderatorCardMiddle">
        <div v-if="loaded">
          <div class="flex snipKids gutterHSm rowSm">
            <strong class="txt5">{{ ob.name }}</strong>
            <span class="clrT2">{{ ob.handle ? `@${ob.handle}` : '' }}</span>
          </div>
          <div class="row">
            <div v-if="ob.valid">
              <div class="rowTn clamp2">{{ ob.moderatorInfo.description }}</div>

              <div v-if="ob.modLanguages && ob.modLanguages.length" class="txSm rowTn">
                {{ 
                  ob.modLanguages.length > 1 ? ob.polyT('moderatorCard.languages', { lang: ob.modLanguages[0], smart_count: ob.modLanguages.length -1 }) : ob.modLanguages[0]
                }}
              </div>

              <div class="flex gutterH tx5 detailsRow">
                <div v-if="ob.hasValidCurrency">
                  <div class="flexNoShrink modFee">
                    {{ ob.polyT(`moderatorCard.${ob.moderatorInfo.fee.feeType}`, { amount, percentage: ob.moderatorInfo.fee.percentage }) }}
                  </div>
                  <div>
                    {{ ob.parseEmojis('📍') }}{{ ob.location || ob.polyT('userPage.noLocation') }}
                  </div>
                  <div class="flexExpand flexNoShrink verifiedWrapper js-verifiedMod"></div>
                </div>
                <span v-else class="clrTErr clamp2">{{ ob.polyT('moderatorCard.noCoinSupport') }}</span>
              </div>
            </div>
          </div>
          <div v-if="ob.showPreferredWarning" class="clrTErr note">{{ ob.polyT('moderatorCard.noPreferredSupport', { coins: ob.moderatorInfo.acceptedCurrencies.join(', ') }) }}</div>
          <span v-else class="clrTErr">{{ ob.polyT('moderatorCard.invalid') }}</span>
        </div>
        <div v-else class="flexCol gutterVSm clrTErr">
          <strong class="txt5 noOverflow">{{ ob.peerID }}</strong>
          <span>{{ ob.polyT('moderatorCard.failed') }}</span>
        </div>

      </div>
      <div class="flexNoShrink">
        <div v-if="ob.valid || ob.controlsOnInvalid" class="flexCol gutterV">
          <button v-if="ob.valid" class="btn clrP clrBr clrSh2 selectBtn js-viewBtn">
            {{ ob.polyT('moderatorCard.view') }}
          </button>

          <button v-if="ob.radioStyle" class="btn clrP clrBr clrSh2 selectBtn js-selectBtn" :data-state="ob.selectedState">
            <i class="ion-checkmark showIfSelected clrTEmph1"></i>
            <i class="ion-close showIfDeselected clrTErr"></i>
            <i class="ion-checkmark showIfUnselected clrTEmph1Disabled"></i>
          </button>
        </div>
      </div>
    </div>
  </div>

</template>

<script setup>
const props = defineProps({
  feeLevel: String,
})

// Check to see if the card was created with at least minimum data, not just a peerID, which would indicate a server error.
const loaded = !!ob.name;
/* Disable the card if it is invalid and the controls should be shown, and it is not selected. This allow the user to de-select invalid cards.
  The view should prevent the invalid card from being selected again, disabling it is redundant but important visually. */
const isDisabled = (!ob.valid && !ob.controlsOnInvalid) || (!ob.valid && ob.controlsOnInvalid && ob.selectedState !== 'selected') || !loaded ? 'disabled' : '';
const style = ob.verified ? 'verified clrBrAlert2 clrBAlert2Grad' : '';

const amount = ob.currencyMod.convertAndFormatCurrency(
  ob.moderatorInfo.fee.fixedFee.amount || 0,
  ob.moderatorInfo.fee.fixedFee.currency.code,
  ob.displayCurrency,
  {
    maxDisplayDecimals:
      !ob.currencyMod.isFiatCur(ob.displayCurrency) ? 6 : undefined
  }
)
</script>
<style lang="scss" scoped>
</style>