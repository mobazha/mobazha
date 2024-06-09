<template>
  <div class="settingsModeration">
    <div class="gutterVMd2">
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHCent">
          <h2 class="h3 clrT">{{ ob.polyT('settings.moderationTab.sectionName') }}</h2>
          <ProcessingButton :className="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn js-save ${isSaving ? 'processing' : ''}`"
            @click="save" :btnText="ob.polyT('settings.btnSave')" />
        </div>
        <hr class="clrBr" />
        <h3>{{ ob.polyT('settings.moderationTab.disputeResolution') }}</h3>
        <p class="tx5 clrT2">{{ ob.polyT('settings.moderationTab.disputeResolutionText') }}</p>

        <div class="tabFormWrapper">
          <form class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col3">
                <span class="required">{{ ob.polyT('settings.moderationTab.moderationStatus') }}</span>
              </div>
              <div class="col9">
                <div class="btnStrip">
                  <div class="btnRadio clrBr">
                    <input type="radio"
                      v-model="formData.moderator"
                      :value="true"
                      id="settingsModerationStatusTrue">
                    <label for="settingsModerationStatusTrue">{{ ob.polyT('settings.on') }}</label>
                  </div>
                  <div class="btnRadio clrBr">
                    <input type="radio"
                      v-model="formData.moderator"
                      :value="false"
                      id="settingsModerationStatusFalse">
                    <label for="settingsModerationStatusFalse">{{ ob.polyT('settings.off') }}</label>
                  </div>
                </div>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label class="required" for="moderationFeeType">{{ ob.polyT('settings.moderationTab.feeTypeAmount') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="feeErrors.length" :errors="feeErrors" />
                <div class="flexRow gutterH rowTn">
                  <div class="col4">
                    <Select2 id="moderationFeeType" v-model="formData.moderatorInfo.fee.feeType" :options="{ minimumResultsForSearch: Infinity }" class="clrBr clrP clrSh2">
                      <option :value="ob.feeTypes.PERCENTAGE" :selected="!formData.moderatorInfo.fee || (formData.moderatorInfo.fee.feeType === ob.feeTypes.PERCENTAGE)">
                        {{ ob.polyT('settings.moderationTab.percentage') }}
                      </option>
                      <option :value="ob.feeTypes.FIXED" :selected="formData.moderatorInfo.fee.feeType === ob.feeTypes.FIXED">
                        {{ ob.polyT('settings.moderationTab.fixed') }}
                      </option>
                      <option :value="ob.feeTypes.FIXED_PLUS_PERCENTAGE" :selected="formData.moderatorInfo.fee.feeType === ob.feeTypes.FIXED_PLUS_PERCENTAGE">
                        {{ ob.polyT('settings.moderationTab.fixedPlusPercentage') }}
                      </option>
                    </Select2>
                  </div>
                  <template v-if="formData.moderatorInfo.fee && formData.moderatorInfo.fee.feeType !== ob.feeTypes.PERCENTAGE">
                    <div class="col2 js-feeFixedInput">
                      <input
                        type="number"
                        class="noSpin clrBr clrSh2"
                        v-model="formData.moderatorInfo.fee.fixedFee.amount"
                        data-var-type="bignumber"
                        placeholder="0.00">
                    </div>
                    <div class="col4 js-feeFixedInput">
                      <Select2 id="moderationCurrency" v-model="formData.moderatorInfo.fee.fixedFee.currency.code" class="clrBr clrP clrSh2" style="width: 100%">
                        <template v-for="currency in currencyList" :key="currency.code">
                          <option :value="currency.code" :selected="currency.code === ccode">{{ currency.nameWithCode }}</option>
                        </template>
                      </Select2>
                    </div>
                  </template>
                  <template v-else-if="formData.moderatorInfo.fee && formData.moderatorInfo.fee.feeType !== ob.feeTypes.FIXED">
                    <div class="col2 js-feePercentageInput">
                      <div class="inputPercentWrapper clrBr clrSh2">
                        <input type="number" maxlength="5" v-model="formData.moderatorInfo.fee.percentage" placeholder="0">
                      </div>
                    </div>
                  </template>
                </div>
                <div class="tx6 txPlaceholder">
                  {{ ob.polyT('settings.moderationTab.feeTypeHelper') }}
                </div>
              </div>

            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsModeratorDescription" class="required">{{
                  ob.polyT('settings.moderationTab.description') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['moderatorInfo.description']" :errors="ob.errors['moderatorInfo.description']" />
                <textarea rows="3" :maxlength="ob.max.description" v-model="formData.moderatorInfo.description"
                  id="settingsModeratorDescription" class="clrBr clrSh2"
                  :placeholder="ob.polyT('settings.moderationTab.descriptionHelper')">{{ ob.description }}</textarea>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsModeratorTerms" class="required">{{ ob.polyT('settings.moderationTab.terms') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['moderatorInfo.termsAndConditions']" :errors="ob.errors['moderatorInfo.termsAndConditions']" />
                <textarea rows="3" :maxlength="ob.max.terms" v-model="formData.moderatorInfo.termsAndConditions"
                  id="settingsModeratorTerms" class="resizable clrBr clrSh2"
                  :placeholder="ob.polyT('settings.moderationTab.termsHelper')"></textarea>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsModeratorTerms" class="required">{{ ob.polyT('settings.moderationTab.languages') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['moderatorInfo.languages']" :errors="ob.errors['moderatorInfo.languages']" />
                <select ref="moderationLanguageSelect" multiple class="clrBr clrP clrSh2"></select>
                <div class="tx6 txPlaceholder">
                  {{ ob.polyT('settings.moderationTab.languagesHelper') }}
                </div>
              </div>
            </div>
          </form>
          <!-- the elements below are not part of the data being saved to the server -->
          <div class="box padMd">
            <div class="flexRow gutterH">
              <div class="col3"></div>
              <div class="col9">
                <ul class="unstyled errorList js-moderationConfirmError" v-show="!hideModerationConfirmError">
                  <li><i class="ion-alert-circled"></i> {{ ob.polyT('settings.moderationTab.errors.confirm') }}</li>
                </ul>
                <input type="checkbox" id="acceptGuidelines" v-model="acceptGuidelinesChecked">
                <label class="tx5b" for="acceptGuidelines">
                  <span v-html='ob.polyT("settings.moderationTab.acceptGuidelines", {
                      acceptGuidelinesLink: `<a
                      href="https://mobazha.org/mobazha-dispute-resolution-guidelines/"
                      class="clrTEm">${ob.polyT("settings.moderationTab.acceptGuidelinesLink")}</a>`
                    })'>
                  </span>
                </label>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3"></div>
              <div class="col9">
                <input type="checkbox" id="understandRequirements" v-model="understandRequirementsChecked">
                <label class="tx5b" for="understandRequirements">
                  <span>{{ ob.polyT('settings.moderationTab.understandRequirements') }}</span>
                </label>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHRight">
          <ProcessingButton :className="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph js-save ${isSaving ? 'processing' : ''}`" @click="save"
            :btnText="ob.polyT('settings.btnSave')" />
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import bigNumber from 'bignumber.js';
import '../../../../backbone/utils/lib/selectize';
import app from '../../../../backbone/app';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import Moderator from '../../../../backbone/models/profile/Moderator';
import { feeTypes } from '../../../../backbone/models/profile/Fee';
import { getTranslatedLangs } from '../../../../backbone/data/languages';
import { getCurrencies } from '../../../../backbone/data/currencies';
import { toStandardNotation } from '../../../../backbone/utils/number';

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
      isSaving: false,

      profile: undefined,
      profileKey: 0,

      acceptGuidelinesChecked: false,
      understandRequirementsChecked: false,
      hideModerationConfirmError: true,

      formData: {
        moderator: false,

        moderatorInfo: {
          description: '',
          termsAndConditions: '',
          languages: [],
          acceptedCurrencies: [],
          fee: {
            fixedFee: {
              amount: 0,
              currency: {
                code: '',
              }
            },
            percentage: 0,
            feeType: 'PERCENTAGE',
          }
        },
      }
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.moderationLanguageSelect = $(this.$refs.moderationLanguageSelect).selectize({
      maxItems: null,
      valueField: 'code',
      searchField: ['name', 'code'],
      items: this.moderatorInfo.get('languages'),
      options: getTranslatedLangs(),
      render: {
        option: (data) => `<div>${data.name}</div>`,
        item: (data) => `<div>${data.name}</div>`,
      },
      onChange: () => {
        this.formData.moderatorInfo.languages = this.moderationLanguageSelect[0].selectize.items;
      }
    });
  },
  watch: {
    acceptGuidelinesChecked() {
      this.hideModerationConfirmError = true;
    },
    understandRequirementsChecked() {
      this.hideModerationConfirmError = true;
    }
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: this.profile.validationError || {},
        languageList: getTranslatedLangs(),
        defaultCurrency: app.settings.get('localCurrency'),
        max: {
          description: this.moderatorInfo.max.descriptionLength,
          terms: this.moderatorInfo.max.termsLength,
        },
        feeTypes,
      };
    },
    moderatorInfo() {
      let access = this.profileKey;

      let moderatorInfo = this.profile.get('moderatorInfo');
      if (!moderatorInfo) {
        moderatorInfo = new Moderator({
          languages: [app.localSettings.standardizedTranslatedLang()],
        });
        this.profile.set('moderatorInfo', this.moderatorInfo);
      }

      return moderatorInfo;
    },
    feeErrors () {
      const ob = this.ob;
      return Object.keys(ob.errors)
        .filter(errKey => errKey.startsWith('moderatorInfo.fee.'))
        .map(errKey => ob.errors[errKey])
    },
    ccode () {
      let fee = this.formData.moderatorInfo.fee;

      return fee.fixedFee.currency.code ? fee.fixedFee.currency.code : app.settings.get('localCurrency');
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this.profile = app.profile.clone();
      this.profile.on('change', () => this.profileKey += 1);

      // Sync our clone with any changes made to the global profile.
      this.listenTo(app.profile, 'someChange', (md, opts) => {
        this.profile.set(opts.setAttrs);

        this.initFormData();
      });

      // Sync the global profile with any changes we save via our clone.
      this.listenTo(
        this.profile,
        'sync',
        // (md, resp, opts) => app.profile.set(this.profile.toJSON(opts.attrs)));
        (md, resp, opts) => {
          app.profile.set(this.profile.toJSON(opts.attrs));
        },
      );

      this.currencyList = getCurrencies();

      this.listenTo(this.profile, 'sync', () => {
        app.profile.set({
          moderator: this.profile.get('moderator'),
          moderatorInfo: this.profile.get('moderatorInfo').toJSON(),
        });
      });

      this.initFormData();
    },

    initFormData() {
      const modInfo = this.moderatorInfo.toJSON();

      this.formData = {
        moderator: this.profile.get('moderator'),
        moderatorInfo: modInfo,
      }

      this.formData.moderatorInfo.fee.fixedFee.amount = toStandardNotation(modInfo.fee.fixedFee.amount);
    },

    getFormData () {
      const formData = this.formData;
      if (formData.moderatorInfo.fee.fixedFee.amount != null) {
        formData.moderatorInfo.fee.fixedFee.amount = bigNumber(formData.moderatorInfo.fee.fixedFee.amount);
      }
      return formData;
    },

    save () {
      const formData = this.getFormData();

      // The user must check both boxes at the bottom of the page if they want to be a moderator,
      // but the values aren't part of the model, they only exist in the DOM and aren't saved.
      if (formData.moderator && !(this.acceptGuidelinesChecked && this.understandRequirementsChecked)) {
        this.hideModerationConfirmError = false;
        return;
      }
      this.hideModerationConfirmError = true;

      this.profile.set(formData);

      const save = this.profile.save(formData, {
        attrs: formData,
        type: 'PUT',
      });

      if (save) {
        const msg = {
          msg: app.polyglot.t('settings.moderationTab.status.saving'),
          type: 'message',
        };

        const statusMessage = app.statusBar.pushMessage({
          ...msg,
          duration: 9999999999999999,
        });

        save.done(() => {
          statusMessage.update({
            msg: app.polyglot.t('settings.moderationTab.status.done'),
            type: 'confirmed',
          });
        })
          .fail((...args) => {
            const errMsg = args[0] && args[0].responseJSON && args[0].responseJSON.reason || '';

            openSimpleMessage(app.polyglot.t('settings.moderationTab.errors.save'), errMsg);

            statusMessage.update({
              msg: app.polyglot.t('settings.moderationTab.status.fail'),
              type: 'warning',
            });
          }).always(() => {
            this.isSaving = false;

            setTimeout(() => statusMessage.remove(), 3000);
          });
      }

      if (save) {
        this.isSaving = true;
      } else {
        const $firstErr = $('.errorList:visible:first');

        if ($firstErr.length) {
          $firstErr[0].scrollIntoViewIfNeeded();
        } else {
          this.$emit('unrecognizedModelError', this, [this.profile]);
        }
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
