<template>
  <div class="moderatorCard clrBr" @click.stop="click">
    <div :class="`moderatorCardInner clrP ${isDisabled} ${ob.verified ? 'verified clrBrAlert2 clrBAlert2Grad' : ''}`">
      <div class="flexRow gutterH moderatorCardContent">
        <template v-if="ob.radioStyle">
          <div class="flexNoShrink">
            <div class="btnRadio">
              <!-- // the card state may be set on render or set on the fly by the view -->
              <div
                tabindex="0"
                :class="`fauxRadioBtn js-selectBtn ${ob.selectedState === 'selected' ? 'active' : 'inactive'}`"
                :data-state="ob.selectedState"
              ></div>
            </div>
          </div>
        </template>
        <div class="flexNoShrink">
          <a class="userIcon disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(ob.avatarHashes)"></a>
        </div>
        <div class="moderatorCardMiddle">
          <template v-if="loaded">
            <div class="flex snipKids gutterHSm rowSm">
              <strong class="txt5">{{ ob.name }}</strong>
              <span class="clrT2">{{ ob.handle ? `@${ob.handle}` : '' }}</span>
            </div>
            <div class="row">
              <template v-if="ob.valid">
                <div class="rowTn clamp2">{{ ob.moderatorInfo.description }}</div>
                <template v-if="ob.modLanguages && ob.modLanguages.length">
                  <div class="txSm rowTn">
                    {{
                      ob.modLanguages.length > 1
                        ? ob.polyT('moderatorCard.languages', { lang: ob.modLanguages[0], smart_count: ob.modLanguages.length - 1 })
                        : ob.modLanguages[0]
                    }}
                  </div>
                </template>
                <div class="flex gutterH tx5 detailsRow">
                  <template v-if="ob.hasValidCurrency">
                    <div class="flexNoShrink modFee">
                      {{ ob.polyT(`moderatorCard.${ob.moderatorInfo.fee.feeType}`, { amount, percentage: ob.moderatorInfo.fee.percentage }) }}
                    </div>
                    <div v-html="`${ ob.parseEmojis('📍') }${ ob.location || ob.polyT('userPage.noLocation') }`"></div>
                    <div class="flexExpand flexNoShrink verifiedWrapper js-verifiedMod"></div>
                  </template>

                  <template v-else>
                    <span class="clrTErr clamp2">{{ ob.polyT('moderatorCard.noCoinSupport') }}</span>
                  </template>
                </div>
                <template v-if="ob.showPreferredWarning">
                  <div class="clrTErr note">{{ ob.polyT('moderatorCard.noPreferredSupport', { coins: ob.moderatorInfo.acceptedCurrencies.join(', ') }) }}</div>
                </template>
              </template>

              <template v-else>
                <span class="clrTErr">{{ ob.polyT('moderatorCard.invalid') }}</span>
              </template>
            </div>
          </template>

          <template v-else>
            <div class="flexCol gutterVSm clrTErr">
              <strong class="txt5 noOverflow">{{ ob.peerID }}</strong>
              <span>{{ ob.polyT('moderatorCard.failed') }}</span>
            </div>
          </template>
        </div>
        <div class="flexNoShrink">
          <template v-if="ob.valid || ob.controlsOnInvalid">
            <div class="flexCol gutterV">
              <template v-if="ob.valid">
                <button class="btn clrP clrBr clrSh2 selectBtn" @click.stop="clickModerator">
                  {{ ob.polyT('moderatorCard.view') }}
                </button>
              </template>
              <template v-if="!ob.radioStyle">
                <button class="btn clrP clrBr clrSh2 selectBtn js-selectBtn" :data-state="ob.selectedState">
                  <i class="ion-checkmark showIfSelected clrTEmph1"></i>
                  <i class="ion-close showIfDeselected clrTErr"></i>
                  <i class="ion-checkmark showIfUnselected clrTEmph1Disabled"></i>
                </button>
              </template>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import loadTemplate from '../../../../backbone/utils/loadTemplate';
import app from '../../../../backbone/app';
import Profile from '../../../../backbone/models/profile/Profile';
import VerifiedMod, { getModeratorOptions } from '../VerifiedMod';
import { handleLinks } from '../../../../backbone/utils/dom';
import { launchModeratorDetailsModal } from '../../../../backbone/utils/modalManager';
import { anySupportedByWallet } from '../../../../backbone/data/walletCurrencies';
import { getLangByCode } from '../../../../backbone/data/languages';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      _state: {
        selectedState: 'unselected',
        preferredCurs: [],
      }
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.render();
  },
  computed: {
    ob() {
      const showPreferredWarning = this._state.preferredCurs.length && !this.hasPreferredCur;

      const verifiedMod = app.verifiedMods.get(this.model.get('peerID'));
      return {
        ...this.templateHelpers,
        displayCurrency: app.settings.get('localCurrency'),
        valid: this.model.isModerator,
        hasValidCurrency: anySupportedByWallet(this.modCurs),
        radioStyle: this.options.radioStyle,
        controlsOnInvalid: this.options.controlsOnInvalid,
        showPreferredWarning,
        verified: !!verifiedMod,
        modLanguages: this.modLanguages,
        ...this.model.toJSON(),
        ...this._state,
      };
    },
    loaded() {
      return !!this.ob.name;
    },
    isDisabled() {
      const ob = this.ob;
      /* Disable the card if it is invalid and the controls should be shown, and it is not selected. This allow the user to de-select invalid cards.
      The view should prevent the invalid card from being selected again, disabling it is redundant but important visually. */
      return (!ob.valid && !ob.controlsOnInvalid) || (!ob.valid && ob.controlsOnInvalid && ob.selectedState !== 'selected') || !this.loaded ? 'disabled' : '';
    },
    amount() {
      const ob = this.ob;
      return ob.currencyMod.convertAndFormatCurrency(
        ob.moderatorInfo.fee.fixedFee.amount || 0,
        ob.moderatorInfo.fee.fixedFee.currency.code,
        ob.displayCurrency,
        {
          maxDisplayDecimals: !ob.currencyMod.isFiatCur(ob.displayCurrency) ? 6 : undefined,
        }
      );
    },
    hasPreferredCur() {
      const preCur = _.intersection(this._state.preferredCurs, this.modCurs);
      return !!preCur.length;
    },
  },
  methods: {
    loadData(options = {}) {
      /* There are 3 valid card selected states:
       selected: This mod is pre-selected, or was activated by the user.
       unselected: Neutral. No action has been taken by the user on this mod.
       deselected: The user has rejected or turned off this mod.
       */
      const validSelectedStates = ['selected', 'unselected', 'deselected'];

      if (!validSelectedStates.includes(options.initialState.selectedState)) {
        throw new Error('Please provide a valid selected state.');
      }

      const opts = {
        radioStyle: false,
        controlsOnInvalid: false,
        notSelected: 'unselected',
        ...options,
        initialState: {
          selectedState: 'unselected',
          preferredCurs: [],
          ...options.initialState,
        },
      };

      this.baseInit(opts);
      if (!this.model || !(this.model instanceof Profile)) {
        throw new Error('Please provide a Profile model.');
      }

      const modInfo = this.model.get('moderatorInfo');
      this.modCurs = (modInfo && modInfo.get('acceptedCurrencies')) || [];

      this.modLanguages = [];
      if (this.model.isModerator) {
        this.modLanguages = this.model
          .get('moderatorInfo')
          .get('languages')
          .map((lang) => {
            const langData = getLangByCode(lang);
            return (langData && langData.name) || lang;
          });
      }

      handleLinks(this.$el);
    },

    clickModerator(e) {
      const modModal = launchModeratorDetailsModal({
        model: this.model,
        purchase: this.options.purchase,
        cardState: this.getState('selectedState'),
      });
      this.listenTo(modModal, 'addAsModerator', () => {
        this.changeSelectState('selected');
      });
    },

    click(e) {
      this.rotateSelectState();
    },

    rotateSelectState() {
      if (this._state.selectedState === 'selected' && !this.options.radioStyle) {
        this.changeSelectState(this.options.notSelected);
      } else if (this.model.isModerator && anySupportedByWallet(this.modCurs)) {
        /* Only change to selected if this is a valid moderator and the user's currency is supported.
        Moderators that have become invalid may be displayed, and can be de-selected to remove them.
        */
        this.changeSelectState('selected');
      }
    },

    changeSelectState(selectedState) {
      if (selectedState !== this._state.selectedState) {
        this.setState({ selectedState });
        this.trigger('modSelectChange', {
          selected: selectedState === 'selected',
          guid: this.model.id,
        });
      }
    },

    render() {
      super.render();

      const showPreferredWarning = this._state.preferredCurs.length && !this.hasPreferredCur;

      const verifiedMod = app.verifiedMods.get(this.model.get('peerID'));

      loadTemplate('components/moderators/card.html', (t) => {
        this.$el.html(
          t({
            displayCurrency: app.settings.get('localCurrency'),
            valid: this.model.isModerator,
            hasValidCurrency: this.hasValidCurrency,
            radioStyle: this.options.radioStyle,
            controlsOnInvalid: this.options.controlsOnInvalid,
            showPreferredWarning,
            verified: !!verifiedMod,
            modLanguages: this.modLanguages,
            ...this.model.toJSON(),
            ...this._state,
          })
        );

        if (this.verifiedMod) this.verifiedMod.remove();

        this.verifiedMod = this.createChild(
          VerifiedMod,
          getModeratorOptions({
            model: verifiedMod,
          })
        );
        this.getCachedEl('.js-verifiedMod').append(this.verifiedMod.render().el);
      });

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
