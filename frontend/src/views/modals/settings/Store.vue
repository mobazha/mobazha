<template>
  <div class="settingsStore">
    <div class="gutterVMd2">
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHCent">
          <h2 class="h3 clrT">{{ ob.polyT('settings.storeTab.sectionName') }}</h2>
          <ProcessingButton
            :className="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn js-save ${isSaving ? 'processing' : ''}`"
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
                      v-model="formData.vendor"
                      :value="true"
                      class="js-profileField"
                      id="settingsModerationStatusTrue"
                      data-var-type="boolean"
                    />
                    <label for="settingsModerationStatusTrue">{{ ob.polyT('settings.on') }}</label>
                  </div>
                  <div class="btnRadio clrBr">
                    <input
                      type="radio"
                      v-model="formData.vendor"
                      :value="false"
                      class="js-profileField"
                      id="settingsModerationStatusFalse"
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
                    :options="{
                      currencies: supportedWalletCurs(),
                      sort: true,
                    }"
                    v-model:activeCurs="formData.currencies"
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
                    <div class="js-modListSelected">
                      <Moderators
                        ref="modsSelected"
                        :key="verifiedModsKey"
                        :options="{
                          cardState: 'selected',
                          controlsOnInvalid: true,
                          fetchErrorTitle: ob.polyT('settings.storeTab.errors.selectedModsTitle'),
                          notSelected: 'deselected',
                          showInvalid: true,
                          showSpinner: false,
                        }"
                        :preferredCurs="formData.currencies"
                      />
                    </div>
                  </div>
                  <ul class="unstyled errorList hide js-submitModByIDInputError">
                    <li><i class="ion-alert-circled"></i><span class="js-submitModByIDInputErrorText" v-html="modByIDInputText"></span></li>
                  </ul>
                  <div class="flex gutterH">
                    <input
                      type="text"
                      class="btnHeight clrBr clrP clrSh2 js-submitModByIDInput"
                      v-model="inputModID"
                      :placeholder="ob.polyT('settings.storeTab.moderatorByIDPlaceholder')"
                    />
                    <ProcessingButton
                      className="btn clrP clrBr clrSh2 flexNoShrink js-submitModByID"
                      @click="clickSubmitModByID"
                      :btnText="ob.polyT('settings.storeTab.moderatoryByIDSubmit')"
                    />
                  </div>
                  <div class="js-modListByID">
                    <Moderators
                      ref="modsByID"
                      :key="verifiedModsKey"
                      v-show="showModListByID"
                      :options="{
                        async: false,
                        excludeIDs: currentMods,
                        fetchErrorTitle: ob.polyT('settings.storeTab.errors.modNotFoundTitle'),
                        showInvalid: true,
                        showSpinner: false,
                        wrapperClasses: 'noMin',
                      }"
                      :preferredCurs="formData.currencies"
                      :noModsFound="noModsByIDFound"
                    />
                  </div>
                  <div>
                    <div class="rowLg">
                      <!-- // just a spacer -->
                    </div>
                  </div>
                  <div class="row">
                    <div class="tx5 rowTn">
                      <div class="flexVCentClearMarg">
                        <h5 class="flexExpand">{{ ob.polyT('settings.storeTab.availableModerators') }}</h5>
                        <input type="checkbox" id="storeVerifiedOnly" v-model="showVerifiedOnly" />
                        <label class="tx5b" for="storeVerifiedOnly">{{ ob.polyT('settings.storeTab.verifiedOnly') }}</label>
                      </div>
                    </div>
                    <ul class="unstyled errorList hide js-modListAvailableError">
                      <li><i class="ion-alert-circled"></i><span class="js-modListAvailableErrorText"></span></li>
                    </ul>
                    <div class="js-modListAvailable">
                      <Moderators
                        ref="modsAvailable"
                        :key="verifiedModsKey"
                        v-show="showModListAvailable"
                        :options="{
                          apiPath: 'moderators',
                          excludeIDs: currentMods,
                          fetchErrorTitle: ob.polyT('settings.storeTab.errors.availableModsTitle'),
                          showLoadBtn: true,
                        }"
                        :preferredCurs="formData.currencies"
                        :showVerifiedOnly="showVerifiedOnly"
                      />
                    </div>
                    <div class="noModsAdded flex clrBr js-noModsAdded" v-show="!showModListAvailable">
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
                      :bb="
                        function () {
                          return {
                            model: shipOpt,
                          };
                        }
                      "
                      @click-remove="onRemoveShippingOption"
                    />
                  </template>
                </div>

                <div class="contentBox padMd clrP clrBr clrSh3 tx3 shipOptPlaceholder">
                  <FormError v-if="ob.errors['shippingOptions']" :errors="ob.errors['shippingOptions']" :class="topLevelShipOptErrs" />
                  <h2 class="h4 clrT js-addShipOptSectionHeading">
                    {{ ob.polyT('settings.storeTab.shippingOptions.optionHeading', { listPosition: shippingOptions.length + 1 }) }}
                  </h2>
                  <hr class="clrBr rowMd" />
                  <a class="btn clrBr clrP clrSh2 rowSm" @click="onClickAddShippingOption">{{
                    ob.polyT('settings.storeTab.shippingOptions.btnAddShippingOption')
                  }}</a>
                  <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperShipping') }}</div>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>

      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHRight">
          <ProcessingButton :className="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph js-save ${isSaving ? 'processing' : ''}`" @click="save" :btnText="ob.polyT('settings.btnSave')" />
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
import ShippingOptionMd from '../../../../backbone/models/settings/ShippingOption';
import Service from '../../../../backbone/models/settings/Service';
import ShippingOption from './ShippingOption.vue';

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

      formData: {
        vendor: false,
        currencies: [...new Set(app.profile.get('currencies'))],
      },

      currentMods: [],

      inputModID: '',

      showModListByID: false,
      showModListAvailable: false,

      showModByIDInputError: false,
      modByIDInputText: '',

      verifiedModsKey: 0,

      shippingOptions: [],

      isSaving: false,
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
      return {
        ...this.templateHelpers,
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

      this.formData.vendor = this.profile.get('vendor');

      this.currentMods = this.settings.get('storeModerators');
      this.showVerifiedOnly = true;

      this.listenTo(app.verifiedMods, 'update', () => this.verifiedModsKey += 1);
      this.shippingOptions = this.settings.get('shippingOptions');
    },
    onClickAddShippingOption() {
      this.shippingOptions.push(new ShippingOptionMd({
        services: [
          new Service(),
        ],
      }));
    },
    hasPhysicalListing() {
      const stats = app.profile.get('stats');
      return stats.get('physicalListingCount') > 0;
    },
    onRemoveShippingOption(md) {
      if (this.hasPhysicalListing() && this.shippingOptions.length == 1 && this.shippingOptions.at(0).cid === md.cid) {
        openSimpleMessage(app.polyglot.t('settings.storeTab.shippingOption.error'), app.polyglot.t('settings.storeTab.shippingOption.noShippingOption'));
      } else {
        this.shippingOptions.remove(md);
      }
    },
    onBulkCoinUpdateConfirm() {
      const newCoins = this.formData.currencies;
      if (newCoins.length) {
        bulkCoinUpdate(this.formData.currencies);
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

    noModsByIDFound({guids}) {
      const modsNotFound = app.polyglot.t('settings.storeTab.errors.modsNotFound', { guids, smart_count: guids.length });
      this.showModByIDError(modsNotFound);
      if (this.$refs.modsByID.modCount === 0) {
        this.showModListByID = false;
      }
    },

    fetchAvailableModerators() {
      // get the verified mods via POST
      this.$refs.modsAvailable.getModeratorsByID({
        moderatorIDs: app.verifiedMods.pluck('peerID'),
        useCache: true,
        method: 'POST',
        apiPath: 'fetchprofiles',
      });
      // get random mods via GET
      this.$refs.modsAvailable.getModeratorsByID();
      this.showModListAvailable = true;
    },

    showModByIDError(msg) {
      this.showModByIDInputError = true;
      this.modByIDInputText = msg;
    },

    clickSubmitModByID() {
      let modID = this.inputModID;

      this.showModByIDInputError = false;

      if (modID) {
        // trim unwanted copy and paste characters
        modID = modID.replace('ob://', '');
        modID = modID.split('/')[0];
        modID = modID.trim();

        if (isIPFS.multihash(modID)) {
          if (!this.currentMods.includes(modID)) {
            if (modID !== app.profile.id) {
              this.$refs.modsByID.getModeratorsByID({ moderatorIDs: [modID] });
              this.showModListByID = true;
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

    getSettingsData() {
      let selected = app.settings.get('storeModerators');
      // The mods may not have loaded in the interface yet. Subtract only explicitly de-selected ones.
      selected = _.without(selected, ...this.$refs.modsSelected.unselectedIDs);
      const byID = this.$refs.modsByID.selectedIDs;
      const available = this.$refs.modsAvailable.selectedIDs;
      return {
        storeModerators: [...new Set([...selected, ...byID, ...available])],
        shippingOptions: this.shippingOptions.toJSON(),
      };
    },

    save() {
      (this.$refs.shippingOptionViews ?? []).forEach((shippingOptionVw) => shippingOptionVw.setModelData());

      // this view saves to two different models
      const profileFormData = this.formData;
      const settingsFormData = this.getSettingsData();

      this.profile.set(profileFormData);
      this.profile.set(profileFormData, { validate: true });
      this.settings.set(settingsFormData);
      this.settings.set(settingsFormData, { validate: true });

      if (!this.profile.validationError && !this.settings.validationError) {
        this.isSaving = true;

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
            const unSel = this.$refs.modsSelected.unselectedIDs;
            const remSel = this.$refs.modsSelected.removeModeratorsByID(unSel);
            const remByID = this.$refs.modsByID.removeModeratorsByID(this.$refs.modsByID.selectedIDs);
            const remAvail = this.$refs.modsAvailable.removeModeratorsByID(this.$refs.modsAvailable.selectedIDs);

            this.$refs.modsByID.excludeIDs = this.currentMods;
            this.$refs.modsByID.moderatorsStatus.setState({
              hidden: true,
            });

            this.$refs.modsSelected.moderatorsCol.add([...remByID, ...remAvail]);
            this.$refs.modsSelected.moderatorsStatus.setState({
              hidden: true,
            });

            this.$refs.modsAvailable.excludeIDs = this.currentMods;
            this.$refs.modsAvailable.moderatorsCol.add(remSel);
            this.$refs.modsAvailable.moderatorsStatus.setState({
              hidden: false,
              total: this.$refs.modsAvailable.modCount,
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
            this.isSaving = false;
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
      if (!this.$refs.modsSelected.modFetches.length) {
        this.$refs.modsSelected.getModeratorsByID({ moderatorIDs: this.currentMods });
      }

      this.showModListByID = !!this.$refs.modsByID.allIDs.length;

      this.showModListAvailable = !!this.$refs.modsAvailable.allIDs.length;

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
