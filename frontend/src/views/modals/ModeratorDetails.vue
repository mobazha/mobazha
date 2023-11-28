<template>
  <div class="modal moderatorDetails modalTop modalScrollPage modalNarrow">
    <BaseModal @close="$emit('close')">
      <template v-slot:component>

        <div class="topControls flex js-closeClickTarget"></div>
        <div class="js-closeClickTarget">
          <div class="flexColRows gutterV">
            <div class="contentBox mDetailWrapper padMd2 clrP clrBr clrSh3 tx5">
              <div class="flex gutterH row">
                <div class="flexNoShrink">
                  <a class="userIcon disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(ob.avatarHashes)" :href="`#${ob.peerID}`"></a>
                </div>
                <div>
                  <div class="flex snipKids gutterHSm rowSm">
                    <strong class="txt5">{{ ob.name }}</strong>
                    <span class="clrT2">{{ ob.handle ? `@${ob.handle}` : '' }}</span>
                  </div>
                  <div class="row clrT2">
                    <p v-html="ob.moderatorInfo.description"></p>
                  </div>
                </div>
              </div>
              <!-- // Don't include the social buttons if this is the viewer's own moderator details -->
              <div class="js-socialBtns">
                <SocialBtns
                  v-if="!ob.isOwnProfile"
                  :options="{
                  targetID: model.id,
                  initialState: {
                    stripClasses: 'flexHCent gutterH',
                    btnClasses: 'clrP clrBr clrSh2',
                  },
                }" />
              </div>
            </div>
            <div class="contentBox mDetailWrapper flexRow flexVCent gutterH padMd2 clrP clrBr clrSh3">
              <div class="mDetail">
                <div class="flexCol flexCent">
                  <div class="txCtr"
                    v-html="`${ob.parseEmojis('📍')} ${ob.location || ob.polyT('userPage.noLocation')}`"></div>
                  <div class="txCtr tx5b clrT2">{{ ob.polyT('moderatorDetails.location') }}</div>
                </div>
              </div>
              <div class="rowDivV clrBrBk TODO"></div>
              <div class="mDetail TODO">
                <div class="flexCol flexCent">
                  <div v-html="ob.parseEmojis('👍')" /> XX <!-- // placeholder for reputation -->
                  <div class="txCtr tx5b clrT2">{{ ob.polyT('moderatorDetails.recommendations') }}</div>
                </div>
              </div>
              <div class="rowDivV clrBrBk"></div>
              <div class="mDetail">
                <div class="flexCol flexCent">
                  <div class="txCtr">{{ ob.polyT(`moderatorCard.${ob.moderatorInfo.fee.feeType}`, {
                    amount: ob.moderatorInfo.fee === 'FIXED' ? ob.currencyMod.convertAndFormatCurrency(ob.moderatorInfo.fee.fixedFee.amount, ob.moderatorInfo.fee.fixedFee.currency.code, ob.displayCurrency) : 0,
                    percentage: ob.moderatorInfo.fee.percentage
                  }) }}</div>
                  <div class="txCtr tx5b clrT2">{{ ob.polyT('moderatorDetails.serviceCharge') }}</div>
                </div>
              </div>
              <div class="rowDivV clrBrBk"></div>
              <div :class="`box mDetail verifiedWrapper ${verifiedModModel ? 'clrBrAlert2 clrBAlert2Grad' : 'clrBrInvis'} js-verifiedMod`">
                <VerifiedMod
                  :key="verfiedModsKey"
                  :options="getModeratorOptions({
                    model: verifiedModModel,
                    shortText: false,
                  })"
                />
              </div>
            </div>
            <div class="contentBox mDetailWrapper padMd2 clrP clrBr clrSh3 tx5">
              <div class="rowMd">
                <h4>{{ ob.polyT('moderatorDetails.currenciesSupported') }}</h4>
                <div ref="supportedCurrenciesList" class="js-supportedCurrenciesList">
                  <SupportedCurrenciesList :options="{
                    initialState: {
                      currencies: model.get('moderatorInfo').get('acceptedCurrencies'),
                    },
                  }"/>
                </div>
              </div>
              <div class="rowLg">
                <h4>{{ ob.polyT('moderatorDetails.languages') }}</h4>
                <template v-for="(lang, j) in ob.modLanguages" :key="j">
                  <div class="rowSm txSm">{{ lang }}</div>
                </template>
              </div>
              <div class="row">
                <h4>{{ ob.polyT('moderatorDetails.termsOfService') }}</h4>
                <p>{{ ob.moderatorInfo.termsAndConditions }}</p>
                <hr class="clrBr">
              </div>
              <div class="flexCol flexHCent gutterV">
                <template v-if="ob.cardState !== 'selected' && (!ob.ownMod || ob.purchase)">
                  <template v-if="ob.crypto.anySupportedByWallet(ob.moderatorInfo.acceptedCurrencies)">
                    <button class="btn clrP clrBr" @click="addAsModerator">
                      <template v-if="ob.purchase">
                        {{ ob.polyT('purchase.chooseModerator') }}
                      </template>

                      <template v-else>
                        {{ ob.polyT('moderatorDetails.addAsModeratorButton') }}
                      </template>
                    </button>
                  </template>

                  <template v-else>
                    <strong>
                      {{ ob.polyT('moderatorDetails.noCoinSupport') }}
                    </strong>
                  </template>
                </template>
                <div>
                  <em class="tx6 clrT2">{{ ob.polyT('moderatorDetails.disclaimer') }}</em>
                </div>
              </div>
            </div>
          </div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../backbone/app';
import Profile from '../../../backbone/models/profile/Profile';
import { getLangByCode } from '../../../backbone/data/languages';

import { getModeratorOptions } from '@/utils/verifiedMod';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      purchase: false,
      cardState: {},

      verfiedModsKey: 0,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      const modLanguages = this.model.get('moderatorInfo')
        .get('languages')
        .map((lang) => {
          const langData = getLangByCode(lang);
          return (langData && langData.name) || lang;
        });
      
      return {
        ...this.templateHelpers,
        displayCurrency: app.settings.get('localCurrency'),
        ownMod: app.settings.get('storeModerators').indexOf(this.model.id) !== -1,
        purchase: this.purchase,
        cardState: this.cardState,
        modLanguages,
        isOwnProfile: this.model.get('peerID') !== app.profile.id,
        ...this.model.toJSON(),
      };
    },

    verifiedModModel() {
      let access = this.verfiedModsKey;
      return app.verifiedMods.get(this.model.get('peerID'));
    }
  },
  methods: {
    getModeratorOptions,

    loadData (options = {}) {
      this.baseInit(options);

      if (!this.model || !(this.model instanceof Profile)) {
        throw new Error('Please provide a Profile model.');
      }

      this.listenTo(app.verifiedMods, 'update', () => {
        this.verfiedModsKey += 1;
      });
    },

    addAsModerator () {
      this.$emit('addAsModerator');
      this.close();
    },

    close() {
      this.$emit('close');
    },
  }
}
</script>
<style lang="scss" scoped></style>
