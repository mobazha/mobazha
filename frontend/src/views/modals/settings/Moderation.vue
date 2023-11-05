<template>
  <div class="settingsModeration">
    <div class="gutterVMd2">
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHCent">
          <h2 class="h3 clrT">{{ ob.polyT('settings.moderationTab.sectionName') }}</h2>
          <ProcessingButton className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn js-save"
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
                      name="moderator"
                      value="true"
                      id="settingsModerationStatusTrue"
                      data-var-type="boolean"
                      :checked="ob.isModerator">
                    <label for="settingsModerationStatusTrue">{{ ob.polyT('settings.on') }}</label>
                  </div>
                  <div class="btnRadio clrBr">
                    <input type="radio"
                      name="moderator"
                      value="false"
                      id="settingsModerationStatusFalse"
                      data-var-type="boolean"
                      :checked="!ob.isModerator">
                    <label for="settingsModerationStatusFalse">{{ ob.polyT('settings.off') }}</label>
                  </div>
                </div>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label class="required" for="moderationFeeType">{{ ob.polyT('settings.moderationTab.feeTypeAmount')
                }}</label>
              </div>
              <div class="col9">
                <FormError v-if="feeErrors.length" :errors="feeErrors" />
                <div class="flexRow gutterH rowTn">
                  <div class="col4">
                    <Select2 id="moderationFeeType" @change="changeFeeType(val)" name="moderatorInfo.fee.feeType" :options="{ minimumResultsForSearch: Infinity }"
                      class="clrBr clrP clrSh2">
                      <option :value="ob.feeTypes.PERCENTAGE"
                        :selected="!ob.fee || (ob.fee && ob.fee.feeType === ob.feeTypes.PERCENTAGE)">
                        {{ ob.polyT('settings.moderationTab.percentage') }}
                      </option>
                      <option :value="ob.feeTypes.FIXED" :selected="ob.fee && ob.fee.feeType === ob.feeTypes.FIXED">
                        {{ ob.polyT('settings.moderationTab.fixed') }}
                      </option>
                      <option :value="ob.feeTypes.FIXED_PLUS_PERCENTAGE"
                        :selected="ob.fee && ob.fee.feeType === ob.feeTypes.FIXED_PLUS_PERCENTAGE">
                        {{ ob.polyT('settings.moderationTab.fixedPlusPercentage') }}
                      </option>
                    </Select2>
                  </div>
                  <div
                    :class="`col2 js-feeFixedInput ${!ob.fee || (ob.fee && ob.fee.feeType === ob.feeTypes.PERCENTAGE) ? 'visuallyHidden' : ''}`">
                    <input
                      type="number"
                      class="noSpin clrBr clrSh2"
                      name="moderatorInfo.fee.fixedFee.amount"
                      data-var-type="bignumber"
                      placeholder="0.00"
                      :value="ob.fee && ob.fee.fixedFee ? ob.number.toStandardNotation(ob.fee.fixedFee.amount) : ''">
                  </div>
                  <div
                    :class="`col4 js-feeFixedInput ${!ob.fee || (ob.fee && ob.fee.feeType === ob.feeTypes.PERCENTAGE) ? 'visuallyHidden' : ''}`">
                    <Select2 id="moderationCurrency" name="moderatorInfo.fee.fixedFee.currency.code"
                      class="clrBr clrP clrSh2" style="width: 100%">
                      <template v-for="(currency, j) in ob.currencyList" :key="j">
                        <option :value="currency.code" :selected="currency.code === ccode">{{ currency.nameWithCode }}
                        </option>
                      </template>
                    </Select2>
                  </div>
                  <div
                    :class="`col2 js-feePercentageInput ${ob.fee && ob.fee.feeType === 'FIXED' ? 'visuallyHidden' : ''}`">
                    <div class="inputPercentWrapper clrBr clrSh2">
                      <input type="text" maxlength="5" name="moderatorInfo.fee.percentage" :value="ob.fee.percentage"
                        placeholder="0" data-var-type="number">
                    </div>
                  </div>
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
                <textarea rows="3" :maxlength="ob.max.description" name="moderatorInfo.description"
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
                <textarea rows="3" :maxlength="ob.max.terms" name="moderatorInfo.termsAndConditions"
                  id="settingsModeratorTerms" class="resizable clrBr clrSh2"
                  :placeholder="ob.polyT('settings.moderationTab.termsHelper')">{{ ob.termsAndConditions }}</textarea>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsModeratorTerms" class="required">{{ ob.polyT('settings.moderationTab.languages') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['moderatorInfo.languages']" :errors="ob.errors['moderatorInfo.languages']" />
                <select id="moderationLanguageSelect" multiple name="moderatorInfo.languages" class="clrBr clrP clrSh2"></select>
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
                <ul class="unstyled errorList hide js-moderationConfirmError">
                  <li><i class="ion-alert-circled"></i> {{ ob.polyT('settings.moderationTab.errors.confirm') }}</li>
                </ul>
                <input type="checkbox" id="acceptGuidelines" :checked="ob.isModerator">
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
                <input type="checkbox" id="understandRequirements" :checked="ob.isModerator">
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
          <ProcessingButton className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph js-save" @click="save"
            :btnText="ob.polyT('settings.btnSave')" />
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import '../../../../backbone/utils/lib/selectize';
import app from '../../../../backbone/app';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import Moderator from '../../../../backbone/models/profile/Moderator';
import { feeTypes } from '../../../../backbone/models/profile/Fee';
import { getTranslatedLangs } from '../../../../backbone/data/languages';
import { getCurrencies } from '../../../../backbone/data/currencies';


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
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    $('#moderationLanguageSelect').selectize({
      maxItems: null,
      valueField: 'code',
      searchField: ['name', 'code'],
      items: moderator.get('languages'),
      options: getTranslatedLangs(),
      render: {
        option: (data) => `<div>${data.name}</div>`,
        item: (data) => `<div>${data.name}</div>`,
      },
    });
  },
  computed: {
    ob () {
      const moderator = this.profile.get('moderatorInfo');

      return {
        ...this.templateHelpers,
        errors: this.profile.validationError || {},
        isModerator: this.profile.get('moderator'),
        languageList: getTranslatedLangs(),
        defaultCurrency: app.settings.get('localCurrency'),
        currencyList: this.currencyList,
        max: {
          description: this.moderator.max.descriptionLength,
          terms: this.moderator.max.termsLength,
        },
        feeTypes,
        ...moderator.toJSON(),
      };
    },
    feeErrors () {
      const ob = this.ob;
      return Object.keys(ob.errors)
        .filter(errKey => errKey.startsWith('moderatorInfo.fee.'))
        .map(errKey => ob.errors[errKey])
    },
    ccode () {
      const ob = this.ob;
      return ob.fee &&
        ob.fee.fixedFee &&
        ob.fee.fixedFee.currency.code ?
        ob.fee.fixedFee.currency.code : ob.defaultCurrency;
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this.profile = app.profile.clone();

      // Sync our clone with any changes made to the global profile.
      this.listenTo(app.profile, 'someChange', (md, opts) => this.profile.set(opts.setAttrs));

      // Sync the global profile with any changes we save via our clone.
      this.listenTo(
        this.profile,
        'sync',
        // (md, resp, opts) => app.profile.set(this.profile.toJSON(opts.attrs)));
        (md, resp, opts) => {
          app.profile.set(this.profile.toJSON(opts.attrs));
        },
      );

      if (this.profile.get('moderatorInfo')) {
        this.moderator = this.profile.get('moderatorInfo');
      } else {
        this.moderator = new Moderator({
          languages: [app.localSettings.standardizedTranslatedLang()],
        });
        this.profile.set('moderatorInfo', this.moderator);
      }

      this.currencyList = getCurrencies();

      this.listenTo(this.profile, 'sync', () => {
        app.profile.set({
          moderator: this.profile.get('moderator'),
          moderatorInfo: this.profile.get('moderatorInfo').toJSON(),
        });
      });
    },

    getFormDataEx () {
      const fields = this.$el.querySelectorAll('select[name], input[name], textarea[name]');
      return this.getFormData(fields);
    },

    save () {
      const formData = this.getFormDataEx();

      // The user must check both boxes at the bottom of the page if they want to be a moderator,
      // but the values aren't part of the model, they only exist in the DOM and aren't saved.
      if (formData.moderator && !($('#understandRequirements').prop('checked')
        && $('#acceptGuidelines').prop('checked'))) {
        $('.js-moderationConfirmError').removeClass('hide');
        return;
      }

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
            $('.js-save').removeClass('processing');
            setTimeout(() => statusMessage.remove(), 3000);
          });
      }

      // Render so errors are shown / cleared.
      this.render();

      if (save) {
        $('.js-save').addClass('processing');
      } else {
        const $firstErr = $('.errorList:visible:first');

        if ($firstErr.length) {
          $firstErr[0].scrollIntoViewIfNeeded();
        } else {
          this.$emit('unrecognizedModelError', this, [this.profile]);
        }
      }
    },

    changeFeeType (val) {
      const feeType = val;

      $('.js-feePercentageInput').toggleClass('visuallyHidden', feeType === 'FIXED');
      $('.js-feeFixedInput').toggleClass('visuallyHidden', feeType === 'PERCENTAGE');
    },

  }
}
</script>
<style lang="scss" scoped></style>
