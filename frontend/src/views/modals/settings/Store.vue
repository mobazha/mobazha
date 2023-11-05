<template>
  <div class="settingsStore">
    <div class="gutterVMd2">
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHCent">
          <h2 class="h3 clrT">{{ ob.polyT('settings.storeTab.sectionName') }}</h2>
          <ProcessingButton
            className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn js-save"
            @click="save"
            :btnText="ob.polyT('settings.btnSave')"
          />
        </div>
        <hr class="clrBr" />

        <div class="tabFormWrapper">
          <form class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col3">
                <label class="required">{{ ob.polyT('settings.storeTab.storeStatus') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['vendor']" :errors="ob.errors['vendor']" />
                <div class="btnStrip">
                  <div class="btnRadio clrBr">
                    <input
                      type="radio"
                      name="vendor"
                      value="true"
                      class="js-profileField"
                      id="settingsModerationStatusTrue"
                      data-var-type="boolean"
                      :checked="ob.vendor"
                    />
                    <label for="settingsModerationStatusTrue">{{ ob.polyT('settings.on') }}</label>
                  </div>
                  <div class="btnRadio clrBr">
                    <input
                      type="radio"
                      name="vendor"
                      value="false"
                      class="js-profileField"
                      id="settingsModerationStatusFalse"
                      :checked="!ob.vendor"
                      data-var-type="boolean"
                    />
                    <label for="settingsModerationStatusFalse">{{ ob.polyT('settings.off') }}</label>
                  </div>
                </div>
              </div>
            </div>
          </form>
          <div class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col3">
                <label class="required">{{ ob.polyT('settings.storeTab.currencies') }}</label>
                <div class="clrT2 txSm rowSm">{{ ob.polyT('settings.storeTab.currenciesHelper') }}</div>
                <div class="clrT2 txSm">{{ ob.polyT('settings.storeTab.currenciesNote') }}</div>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['currencies']" :errors="ob.errors['currencies']" />
                <div class="row js-currencySelector">
                  <CryptoCurSelector
                    ref="currencySelector"
                    :options="{
                      initialState: {
                        currencies: supportedWalletCurs(),
                        activeCurs: [...new Set(app.profile.get('currencies'))],
                        sort: true,
                      },
                    }"
                    @currencyClicked="handleCurrencyClicked"
                  />
                </div>
                <div class="flexHRight">
                  <div class="js-bulkCoinUpdateBtn">
                    <BulkCoinUpdateBtn ref="bulkCoinUpdateBtn" @bulkCoinUpdateConfirm="onBulkCoinUpdateConfirm" />
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col3">
                <label>{{ ob.polyT('settings.storeTab.moderators') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.storeTab.moderatorsHelper') }}</div>
              </div>
              <div class="col9">
                <div class="flexColRows gutterV">
                  <div class="row">
                    <h5>{{ ob.polyT('settings.storeTab.selectedModerators') }}</h5>
                    <div class="js-modListSelected"></div>
                  </div>
                  <ul class="unstyled errorList hide js-submitModByIDInputError">
                    <li><i class="ion-alert-circled"></i><span class="js-submitModByIDInputErrorText"></span></li>
                  </ul>
                  <div class="flex gutterH">
                    <input
                      type="text"
                      class="btnHeight clrBr clrP clrSh2 js-submitModByIDInput"
                      :placeholder="ob.polyT('settings.storeTab.moderatorByIDPlaceholder')"
                    />
                    <ProcessingButton
                      className="btn clrP clrBr clrSh2 flexNoShrink js-submitModByID"
                      @click="clickSubmitModByID"
                      :btnText="ob.polyT('settings.storeTab.moderatoryByIDSubmit')"
                    />
                  </div>
                  <div class="js-modListByID"></div>
                  <div>
                    <div class="rowLg">
                      <!-- // just a spacer -->
                    </div>
                  </div>
                  <div class="row">
                    <div class="tx5 rowTn">
                      <div class="flexVCentClearMarg">
                        <h5 class="flexExpand">{{ ob.polyT('settings.storeTab.availableModerators') }}</h5>
                        <input type="checkbox" id="storeVerifiedOnly" :checked="ob.showVerifiedOnly" @click="onClickVerifiedOnly" />
                        <label class="tx5b" for="storeVerifiedOnly">{{ ob.polyT('settings.storeTab.verifiedOnly') }}</label>
                      </div>
                    </div>
                    <ul class="unstyled errorList hide js-modListAvailableError">
                      <li><i class="ion-alert-circled"></i><span class="js-modListAvailableErrorText"></span></li>
                    </ul>
                    <div class="js-modListAvailable"></div>
                    <div class="noModsAdded flex clrBr js-noModsAdded" v-show="!ob.modsAvailable.length">
                      <button class="btn clrP clrBr browseMods" @click="fetchAvailableModerators">
                        {{ ob.polyT('settings.storeTab.browseModerators') }}
                      </button>
                    </div>
                  </div>
                  <div class="tx6 clrT2 row">
                    {{ ob.polyT('settings.storeTab.disclaimer') }}
                  </div>
                </div>
                <template v-if="ob.errors.storeModerators">
                  <div class="row"></div>
                  <FormError v-if="ob.errors['storeModerators']" :errors="ob.errors['storeModerators']" />
                </template>
              </div>
            </div>
          </div>
          <div class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col3">
                <label class="required">{{ ob.polyT('editListing.sectionNames.shipping') }}</label>
              </div>
            </div>
            <section ref="sectionShipping" class="shippingSection js-sectionShipping">
              <div class="gutterVMd">
                <div class="js-shippingOptionsWrap shippingOptionsWrap gutterVMd">
                  <template v-for="(shipOpt, shipOptIndex) in shippingOptions" :key="shipOpt.cid">
                    <ShippingOption
                      ref="shippingOptionViews"
                      :options="{
                        getCurrency: () => formData.metadata.pricingCurrency.code,
                        listPosition: shipOptIndex + 1,
                      }"
                      :bb="function() {
                        return {
                          model: shipOpt,
                        }
                      }"
                      @click-remove="onRemoveShippingOption" />
                  </template>
                </div>
                <div class="contentBox padMd clrP clrBr clrSh3 tx3 shipOptPlaceholder">
                  <FormError v-if="ob.errors['shippingOptions']" :errors="ob.errors['shippingOptions']" :class="topLevelShipOptErrs" />
                  <h2 class="h4 clrT js-addShipOptSectionHeading">
                    {{ ob.polyT('editListing.shippingOptions.optionHeading', { listPosition: shippingOptions.length + 1 }) }}
                  </h2>
                  <hr class="clrBr rowMd" />
                  <a class="btn clrBr clrP clrSh2 rowSm" @click="onClickAddShippingOption">{{ ob.polyT('editListing.shippingOptions.btnAddShippingOption') }}</a>
                  <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperShipping') }}</div>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>

      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHRight">
          <ProcessingButton className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph js-save" @click="save" :btnText="ob.polyT('settings.btnSave')" />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import * as isIPFS from 'is-ipfs';
import app from '../../../../backbone/app';
import '../../../../backbone/lib/whenAll.jquery';
import { bulkCoinUpdate } from '../../../../backbone/utils/bulkCoinUpdate';
import { supportedWalletCurs } from '../../../../backbone/data/walletCurrencies';
import Moderators from '../../../../backbone/views/components/moderators/Moderators';
import BulkCoinUpdateBtn from './BulkCoinUpdateBtn.vue';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import ShippingOptions from '../../../../backbone/collections/listing/ShippingOptions.js';
import ShippingOptionMd from '../../../../backbone/models/listing/ShippingOption';
import Service from '../../../../backbone/models/listing/Service';
import Listing from '../../../../backbone/models/listing/Listing';

import ShippingOption from '../editListing/ShippingOption.vue';

export default {
  components: {
    BulkCoinUpdateBtn,
    ShippingOption,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      app: app,
      model: {},

      shippingOptions: new ShippingOptions(),
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.render();
  },
  watch: {
    showVerifiedOnly(val) {
      if (this.modsAvailable) {
        this.modsAvailable.setState({ showVerifiedOnly: val });
      }
    },
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        modsAvailable: this.modsAvailable.allIDs,
        showVerifiedOnly: this.showVerifiedOnly,
        errors: {
          ...(this.profile.validationError || {}),
          ...(this.settings.validationError || {}),
        },
        ...this.profile.toJSON(),
        ...this.settings.toJSON(),
      };
    },
  },
  methods: {
    supportedWalletCurs,
    loadData(options = {}) {
      this.baseInit(options);

      this.profile = app.profile.clone();

      // Sync our clone with any changes made to the global profile.
      this.listenTo(app.profile, 'someChange', (md, pOpts) => this.profile.set(pOpts.setAttrs));

      // Sync the global profile with any changes we save via our clone.
      this.listenTo(this.profile, 'sync', () => app.profile.set(this.profile.toJSON()));

      this.settings = app.settings.clone();

      // Sync our clone with any changes made to the global settings model.
      this.listenTo(app.settings, 'someChange', (md, sOpts) => this.settings.set(sOpts.setAttrs));

      // Sync the global settings model with any changes we save via our clone.
      this.listenTo(this.settings, 'sync', (md, resp, sOpts) => app.settings.set(this.settings.toJSON(sOpts.attrs)));

      const preferredCurs = [...new Set(app.profile.get('currencies'))];

      this.currentMods = this.settings.get('storeModerators');
      this.showVerifiedOnly = true;
      this.model = new Listing({
        shippingOptions: [],
      });

      this.modsSelected = new Moderators({
        cardState: 'selected',
        controlsOnInvalid: true,
        fetchErrorTitle: app.polyglot.t('settings.storeTab.errors.selectedModsTitle'),
        notSelected: 'deselected',
        showInvalid: true,
        showSpinner: false,
        initialState: {
          preferredCurs,
        },
      });

      this.modsByID = new Moderators({
        async: false,
        excludeIDs: this.currentMods,
        fetchErrorTitle: app.polyglot.t('settings.storeTab.errors.modNotFoundTitle'),
        showInvalid: true,
        showSpinner: false,
        wrapperClasses: 'noMin',
        initialState: {
          preferredCurs,
        },
      });

      this.listenTo(this.modsByID, 'noModsFound', (mOpts) => this.noModsByIDFound(mOpts.guids));

      this.modsAvailable = new Moderators({
        apiPath: 'moderators',
        excludeIDs: this.currentMods,
        fetchErrorTitle: app.polyglot.t('settings.storeTab.errors.availableModsTitle'),
        showLoadBtn: true,
        initialState: {
          preferredCurs,
          showVerifiedOnly: true,
        },
      });

      const modsToCheckOnVerifiedUpdate = [
        {
          view: this.modsSelected,
          hasVerifiedMods: app.verifiedMods.matched(this.modsSelected.allIDs).length > 0,
        },
        {
          view: this.modsByID,
          hasVerifiedMods: app.verifiedMods.matched(this.modsByID.allIDs).length > 0,
        },
        {
          view: this.modsAvailable,
          hasVerifiedMods: app.verifiedMods.matched(this.modsAvailable.allIDs).length > 0,
        },
      ];

      this.listenTo(app.verifiedMods, 'update', () => {
        modsToCheckOnVerifiedUpdate.forEach((obj) => {
          const nowSelected = app.verifiedMods.matched(obj.view.allIDs).length > 0;
          if (nowSelected !== obj.hasVerifiedMods) {
            obj.hasVerifiedMods = nowSelected;
            obj.view.render();
          }
        });
      });
    },
    onClickAddShippingOption() {
      this.shippingOptions.push(
        new ShippingOptionMd({
          services: [new Service()],
        })
      );
    },
    onRemoveShippingOption(md) {
      this.shippingOptions.remove(md);
    },
    onBulkCoinUpdateConfirm() {
      const newCoins = this.$refs.currencySelector.getState().activeCurs;
      if (newCoins.length) {
        bulkCoinUpdate(this.$refs.currencySelector.getState().activeCurs);
        this.$refs.bulkCoinUpdateBtn.setState({
          isBulkCoinUpdating: true,
          showConfirmTooltip: false,
          error: '',
        });
      } else {
        this.$refs.bulkCoinUpdateBtn.setState({
          isBulkCoinUpdating: false,
          showConfirmTooltip: false,
          error: 'NoCoinsError',
        });
      }
    },

    noModsByIDFound(guids) {
      const modsNotFound = app.polyglot.t('settings.storeTab.errors.modsNotFound', { guids, smart_count: guids.length });
      this.showModByIDError(modsNotFound);
      if (this.modsByID.modCount === 0) {
        $('.js-modListByID').addClass('hide');
      }
    },

    fetchAvailableModerators() {
      // get the verified mods via POST
      this.modsAvailable.getModeratorsByID({
        moderatorIDs: app.verifiedMods.pluck('peerID'),
        useCache: true,
        method: 'POST',
        apiPath: 'fetchprofiles',
      });
      // get random mods via GET
      this.modsAvailable.getModeratorsByID();
      $('.js-modListAvailable').removeClass('hide');
      $('.js-noModsAdded').addClass('hide');
    },

    showModByIDError(msg) {
      $('.js-submitModByIDInputError').removeClass('hide');
      $('.js-submitModByIDInputErrorText').text(msg);
    },

    handleCurrencyClicked(opts) {
      const preferredCurs = opts.activeCurs;
      this.modsSelected.setState({ preferredCurs });
      this.modsByID.setState({ preferredCurs });
      this.modsAvailable.setState({ preferredCurs });
    },

    clickSubmitModByID() {
      let modID = $('.js-submitModByIDInput').val();

      $('.js-submitModByIDInputError').addClass('hide');

      if (modID) {
        // trim unwanted copy and paste characters
        modID = modID.replace('ob://', '');
        modID = modID.split('/')[0];
        modID = modID.trim();

        if (isIPFS.multihash(modID)) {
          if (!this.currentMods.includes(modID)) {
            if (modID !== app.profile.id) {
              this.modsByID.getModeratorsByID({ moderatorIDs: [modID] });
              $('.js-modListByID').removeClass('hide');
            } else {
              const ownGUID = app.polyglot.t('settings.storeTab.errors.ownGUID', { guid: modID });
              this.showModByIDError(ownGUID);
            }
          } else {
            const dupeGUID = app.polyglot.t('settings.storeTab.errors.dupeGUID', { guid: modID });
            this.showModByIDError(dupeGUID);
          }
        } else {
          const notGUID = app.polyglot.t('settings.storeTab.errors.notGUID', { guid: modID });
          this.showModByIDError(notGUID);
        }
      } else {
        const blankError = app.polyglot.t('settings.storeTab.errors.modIsBlank');
        this.showModByIDError(blankError);
      }
    },

    onClickVerifiedOnly(e) {
      this.showVerifiedOnly = $(e.target).prop('checked');
    },

    getProfileFormData(subset = this.$profileFormFields) {
      return this.getFormData(subset);
    },

    getSettingsData() {
      let selected = app.settings.get('storeModerators');
      // The mods may not have loaded in the interface yet. Subtract only explicitly de-selected ones.
      selected = _.without(selected, ...this.modsSelected.unselectedIDs);
      const byID = this.modsByID.selectedIDs;
      const available = this.modsAvailable.selectedIDs;
      return { storeModerators: [...new Set([...selected, ...byID, ...available])] };
    },

    save() {
      // this view saves to two different models
      const profileFormData = this.getProfileFormData();
      profileFormData.currencies = this.$refs.currencySelector.getState().activeCurs;
      const settingsFormData = this.getSettingsData();

      this.profile.set(profileFormData);
      this.profile.set(profileFormData, { validate: true });
      this.settings.set(settingsFormData);
      this.settings.set(settingsFormData, { validate: true });

      if (!this.profile.validationError && !this.settings.validationError) {
        $('.js-save').addClass('processing');

        const msg = {
          msg: app.polyglot.t('settings.storeTab.status.saving'),
          type: 'message',
        };

        const statusMessage = app.statusBar.pushMessage({
          ...msg,
          duration: 9999999999999999,
        });

        const profileSave = this.profile.save(profileFormData, {
          attrs: profileFormData,
          type: 'PUT',
        });

        const settingsSave = this.settings.save(settingsFormData, {
          attrs: settingsFormData,
          type: 'PUT',
        });

        $.when(profileSave, settingsSave)
          .done(() => {
            // both have saved
            statusMessage.update({
              msg: app.polyglot.t('settings.storeTab.status.done'),
              type: 'confirmed',
            });

            // move the changed moderators
            this.currentMods = this.settings.get('storeModerators');
            const unSel = this.modsSelected.unselectedIDs;
            const remSel = this.modsSelected.removeModeratorsByID(unSel);
            const remByID = this.modsByID.removeModeratorsByID(this.modsByID.selectedIDs);
            const remAvail = this.modsAvailable.removeModeratorsByID(this.modsAvailable.selectedIDs);

            this.modsByID.excludeIDs = this.currentMods;
            this.modsByID.moderatorsStatus.setState({
              hidden: true,
            });

            this.modsSelected.moderatorsCol.add([...remByID, ...remAvail]);
            this.modsSelected.moderatorsStatus.setState({
              hidden: true,
            });

            this.modsAvailable.excludeIDs = this.currentMods;
            this.modsAvailable.moderatorsCol.add(remSel);
            this.modsAvailable.moderatorsStatus.setState({
              hidden: false,
              total: this.modsAvailable.modCount,
              showSpinner: false,
            });

            // If any of the mods moved to the available collect are unverified, show them
            if (app.verifiedMods.matched(unSel).length !== unSel.length) {
              // Don't render, the render is in the always handler
              this.showVerifiedOnly = false;
            }
          })
          .fail((...args) => {
            // if at least one save fails, the save has failed.
            const errMsg = (args[0] && args[0].responseJSON && args[0].responseJSON.reason) || '';
            openSimpleMessage(app.polyglot.t('settings.storeTab.status.error'), errMsg);

            statusMessage.update({
              msg: app.polyglot.t('settings.storeTab.status.fail'),
              type: 'warning',
            });
          })
          .always(() => {
            $('.js-save').removeClass('processing');
            setTimeout(() => statusMessage.remove(), 3000);
            this.render();
          });
      } else {
        const $firstErr = $('.errorList:first:not(.hide)');

        if ($firstErr.length) {
          $firstErr[0].scrollIntoViewIfNeeded();
        } else {
          const models = [];
          if (this.profile.validationError) models.push(this.profile);
          if (this.settings.validationError) models.push(this.settings);
          this.$emit('unrecognizedModelError', this, models);
        }
      }
    },
    render() {
      this.modsSelected.delegateEvents();
      $('.js-modListSelected').append(this.modsSelected.render().el);
      if (!this.modsSelected.modFetches.length) {
        this.modsSelected.getModeratorsByID({ moderatorIDs: this.currentMods });
      }

      this.modsByID.delegateEvents();
      $('.js-modListByID').append(this.modsByID.render().el).toggleClass('hide', !this.modsByID.allIDs.length);

      this.modsAvailable.delegateEvents();
      $('.js-modListAvailable').append(this.modsAvailable.render().el).toggleClass('hide', !this.modsAvailable.allIDs.length);

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
