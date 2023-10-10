<template>
  <div class="settingsGeneral">
    <div class="contentBox padMd clrP clrBr clrSh3">
      <div class="flexHCent">
        <h2 class="h3 clrT">{{ ob.polyT('settings.generalTab.sectionName') }}</h2>
        <ProcessingButton className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn js-save"
          @click="save" :btnText="ob.polyT('settings.btnSave')" />
      </div>
      <hr class="clrBr" />

      <div class="tabFormWrapper">
        <form class="box clrP padMdKids padStack settingsGeneralForm">
          <div class="flexRow gutterH">
            <div class="col3">
              <label class="required">{{ ob.polyT('settings.language') }}</label>
              <div class="clrT2 txSm">{{ ob.polyT('settings.generalTab.helperLanguage') }}</div>
            </div>
            <div class="col6">
              <FormError v-if="ob.errors['language']" :errors="ob.errors['language']" />
              <select id="settingsLanguageSelect" name="language" class="clrSh2" data-persistence-location="local">
                <template v-for="(lang, j) in ob.languageList" :key="j">
                  <option :value="lang.code" :selected="lang.code == ob.language">{{ lang.name }}</option>
                </template>
              </select>
              <div class="clrT2 txSm padSm">{{ ob.polyT('settings.generalTab.helperTranslations', {
                helperTranslationsLink: `<a href="https://www.transifex.com/ob1/openbazaar/"
                  class="clrTEm">${ob.polyT('settings.generalTab.helperTranslationsLink')}</a>`
              }) }}</div>
            </div>
          </div>
          <div class="flexRow gutterH">
            <div class="col3">
              <label class="required">{{ ob.polyT('settings.country') }}</label>
              <div class="clrT2 txSm">{{ ob.polyT('settings.generalTab.helperCountry') }}</div>
            </div>
            <div class="col6">
              <FormError v-if="ob.errors['country']" :errors="ob.errors['country']" />
              <select id="settingsCountrySelect" name="country" class="clrSh2">
                <template v-for="(country, j) in ob.countryList" :key="j">
                  <option :value="country.dataName" :selected="country.dataName == ob.country">{{ country.name }}</option>
                </template>
              </select>
            </div>
          </div>
          <div class="flexRow gutterH">
            <div class="col3">
              <label class="required">{{ ob.polyT('settings.currency') }}</label>
              <div class="clrT2 txSm">{{ ob.polyT('settings.generalTab.helperCurrency') }}</div>
            </div>
            <div class="col6">
              <FormError v-if="ob.errors['localCurrency']" :errors="ob.errors['localCurrency']" />
              <select id="settingsCurrencySelect" @change="onChangeCurrencySelect(val)" name="localCurrency"
                class="clrSh2">
                <template v-for="(currency, j) in ob.currencyList" :key="j">
                  <option :value="currency.code" :selected="currency.code == ob.localCurrency">{{ currency.nameWithCode }}
                  </option>
                </template>
              </select>
            </div>
          </div>
          <div class="flexRow gutterH js-bitcoinUnitField" v-show="!ob.localCurrency !== 'BTC'">
            <div class="col3">
              <label class="required">{{ ob.polyT('settings.generalTab.bitcoinUnit') }}</label>
              <div class="clrT2 txSm"></div>
            </div>
            <div class="col6">
              <FormError v-if="ob.errors['localCurrency']" :errors="ob.errors['localCurrency']" />
              <div class="btnStrip">
                <div class="btnRadio clrBr">
                  <input type="radio" data-persistence-location="local" name="bitcoinUnit" value="BTC"
                    id="settingsBitcoinUnitBtc" :checked="ob.bitcoinUnit === 'BTC'">
                  <label for="settingsBitcoinUnitBtc">{{ ob.polyT('settings.generalTab.bitcoinUnitTypes.BTC') }}</label>
                </div>
                <div class="btnRadio clrBr">
                  <input type="radio" data-persistence-location="local" name="bitcoinUnit" value="MBTC"
                    id="settingsBitcoinUnitMbtc" :checked="ob.bitcoinUnit === 'MBTC'">
                  <label for="settingsBitcoinUnitMbtc">{{ ob.polyT('settings.generalTab.bitcoinUnitTypes.MBTC') }}</label>
                </div>
                <div class="btnRadio clrBr">
                  <input type="radio" data-persistence-location="local" name="bitcoinUnit" value="UBTC"
                    id="settingsBitcoinUnitUbtc" :checked="ob.bitcoinUnit === 'UBTC'">
                  <label for="settingsBitcoinUnitUbtc">{{ ob.polyT('settings.generalTab.bitcoinUnitTypes.UBTC') }}</label>
                </div>
                <div class="btnRadio clrBr">
                  <input type="radio" data-persistence-location="local" name="bitcoinUnit" value="SATOSHI"
                    id="settingsBitcoinUnitSatoshi" :checked="ob.bitcoinUnit === 'SATOSHI'">
                  <label for="settingsBitcoinUnitSatoshi">{{ ob.polyT('settings.generalTab.bitcoinUnitTypes.SATOSHI')
                  }}</label>
                </div>
              </div>
            </div>
          </div>
          <div class="flexRow gutterH">
            <div class="col3">
              <label>{{ ob.polyT('settings.viewNsfwContent') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['showNsfw']" :errors="ob.errors['showNsfw']" />
              <div class="btnStrip">
                <div class="btnRadio clrBr">
                  <input type="radio" name="showNsfw" value="true" id="settingsViewNSFWInputTrue" data-var-type="boolean"
                    :checked="ob.showNsfw">
                  <label for="settingsViewNSFWInputTrue">{{ ob.polyT('settings.yes') }}</label>
                </div>
                <div class="btnRadio clrBr">
                  <input type="radio" name="showNsfw" value="false" id="settingsViewNSFWInputFalse"
                    data-var-type="boolean" :checked="!ob.showNsfw">
                  <label for="settingsViewNSFWInputFalse">{{ ob.polyT('settings.no') }}</label>
                </div>
              </div>
            </div>
          </div>
          <div class="flexRow gutterH TODO">
            <!-- // The design was changed to remove this after it was added, it will be used at some point in the future -->
            <div class="col3">
              <label>{{ ob.polyT('settings.generalTab.verifiedMods') }}</label>
              <div class="clrT2 txSm">
                <a class="txU js-restoreDefaultverifiedModProvider" @click="clickRestoreDefaultVerifiedModProvider">{{
                  ob.polyT('settings.generalTab.restoreDefault') }}</a>
              </div>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['verifiedModsProvider']" :errors="ob.errors['verifiedModsProvider']" />
              <input class="clrP clrBr rowTn js-verifiedModsProvider" data-persistence-location="local" type="text"
                name="verifiedModsProvider" :value="ob.verifiedModsProvider">
            </div>
          </div>
        </form>
      </div>

      <hr class="clrBr" />
      <div class="flexHRight">
        <ProcessingButton className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph js-save" @click="save"
          :btnText="ob.polyT('settings.btnSave')" />
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import { translationLangs } from '../../../../backbone/data/languages';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
import { getCurrencies } from '../../../../backbone/data/currencies';
import { openSimpleMessage } from '../SimpleMessage';


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
    $('#settingsLanguageSelect').select2();
    $('#settingsCountrySelect').select2();
    $('#settingsCurrencySelect').select2();

    this._$settingsFields = null;
    this._$localSettingsFields = null;
    this._$btnSave = null;
    this._$bitcoinUnitField = null;
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        languageList: translationLangs,
        countryList: this.countryList,
        currencyList: this.currencyList,
        errors: {
          ...(this.settings.validationError || {}),
          ...(this.localSettings.validationError || {}),
        },
        ...this._localSettings,
        ...this._settings,
        // local lang should be declared after the server model, so the local
        // lang takes precedence over the deprecated server one
        language: this.localSettings.get('language'),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this.settings = app.settings.clone();

      // Sync our clone with any changes made to the global settings model.
      this.listenTo(app.settings, 'someChange',
        (md, opts) => this.settings.set(opts.setAttrs));

      // Sync the global settings model with any changes we save via our clone.
      this.listenTo(this.settings, 'sync', (md, resp, opts) => app.settings.set(opts.attrs));

      this.localSettings = app.localSettings.clone();

      // Sync our clone with any changes made to the global local settings model.
      this.listenTo(this.localSettings, 'sync',
        () => app.localSettings.set(this.localSettings.toJSON()));

      // Sync the global local settings model with any changes we save via our clone.
      this.listenTo(this.localSettings, 'sync',
        (md, resp, opts) => app.localSettings.set(opts.attrs));

      this.countryList = getTranslatedCountries();
      this.currencyList = getCurrencies();
    },

    onChangeCurrencySelect (val) {
      if (val === 'BTC') {
        this.$bitcoinUnitField.removeClass('hide');
      } else {
        this.$bitcoinUnitField.addClass('hide');
      }
    },

    clickRestoreDefaultVerifiedModProvider () {
      // this is currently hidden in the template because it was taken out of the design for now
      const defaultVal = this.localSettings.defaults().verifiedModsProvider;
      $('.js-verifiedModsProvider').val(defaultVal);
    },

    getFormDataEx (fields = this.$settingsFields) {
      return this.getFormData(fields);
    },

    save () {
      this.localSettings.set(this.getFormDataEx(this.$localSettingsFields));
      this.localSettings.set({}, { validate: true });

      const settingsFormData = this.getFormDataEx();
      this.settings.set(settingsFormData);
      this.settings.set({}, { validate: true });

      if (!this.localSettings.validationError && !this.settings.validationError) {
        const msg = {
          msg: app.polyglot.t('settings.generalTab.statusSaving'),
          type: 'message',
        };

        const statusMessage = app.statusBar.pushMessage({
          ...msg,
          duration: 9999999999999999,
        });

        // let's save and monitor both save processes
        const localSave = this.localSettings.save();
        const serverSave = this.settings.save(settingsFormData, {
          attrs: settingsFormData,
          type: 'PUT',
        });

        $.when(localSave, serverSave)
          .done(() => {
            // both succeeded!
            statusMessage.update({
              msg: app.polyglot.t('settings.generalTab.statusSaveComplete'),
              type: 'confirmed',
            });
          })
          .fail((...args) => {
            // One has failed, the other may have also failed or may
            // fail or may succeed. It doesn't matter, for our purposed one
            // failure is enough for us to consider the "save" to have failed
            const errMsg = args[0] && args[0].responseJSON &&
              args[0].responseJSON.reason || '';

            openSimpleMessage(app.polyglot.t('settings.generalTab.saveErrorAlertTitle'), errMsg);

            statusMessage.update({
              msg: app.polyglot.t('settings.generalTab.statusSaveFailed'),
              type: 'warning',
            });
          })
          .always(() => {
            this.$btnSave.removeClass('processing');
            setTimeout(() => statusMessage.remove(), 3000);
          });
      }

      if (!this.localSettings.validationError && !this.settings.validationError) {
        this.$btnSave.addClass('processing');
      } else {
        const $firstErr = $('.errorList:first');

        if ($firstErr.length) {
          $firstErr[0].scrollIntoViewIfNeeded();
        } else {
          const models = [];
          if (this.localSettings.validationError) models.push(this.localSettings);
          if (this.settings.validationError) models.push(this.settings);
          this.$emit('unrecognizedModelError', this, models);
        }
      }
    },

    get $btnSave () {
      return this._$btnSave ||
        (this._$btnSave = $('.js-save'));
    },

    get $bitcoinUnitField () {
      return this._$bitcoinUnitField ||
        (this._$bitcoinUnitField = $('.js-bitcoinUnitField'));
    },

    get $settingsFields () {
      return this._$settingsFields ||
        (this._$settingsFields =
          $('select[name], input[name], textarea[name]')
            .not('[data-persistence-location="local"]'));
    },

    get $localSettingsFields () {
      return this._$localSettingsFields ||
        (this._$localSettingsFields =
          $('[data-persistence-location="local"]'));
    },
  }
}
</script>
<style lang="scss" scoped></style>
