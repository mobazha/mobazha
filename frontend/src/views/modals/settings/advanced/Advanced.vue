<template>
  <div class="settingsAdvanced">
    <div class="gutterVMd2">
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHCent">
          <h2 class="h3 clrT">{{ ob.polyT('settings.advancedTab.sectionName') }}</h2>
          <ProcessingButton
            className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn js-save"
            @click="save"
            :btnText="ob.polyT('settings.btnSave')" />
        </div>
        <hr class="clrBr" />
        <div class="tabFormWrapper clrS">
          <div class="js-appearanceContainer">
            <form class="box padMdKids padStack clrP clrBr js-appearance">
              <div class="flexRow gutterH TODO">
                <div class="col3">
                  <label>{{ ob.polyT('settings.advancedTab.appearance.visualEffects') }}</label>
                  <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.helperVisualEffects') }}</div>
                </div>
                <div class="col9">
                  <div class="btnStrip">
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="showAdvancedVisualEffects"
                        value="true"
                        id="advancedTabVisualEffectsOn"
                        data-var-type="boolean"
                        :checked="ob.showAdvancedVisualEffects">
                      <label for="advancedTabVisualEffectsOn">{{ ob.polyT('settings.on') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="showAdvancedVisualEffects"
                        value="false"
                        id="advancedTabVisualEffectsOff"
                        data-var-type="boolean"
                        :checked="!ob.showAdvancedVisualEffects">
                      <label for="advancedTabVisualEffectsOff">{{ ob.polyT('settings.off') }}</label>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flexRow gutterH">
                <div class="col3">
                  <label>{{ ob.polyT('settings.advancedTab.appearance.windowControlStyle') }}</label>
                  <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.helperWindowControls') }}</div>
                </div>
                <div class="col9">
                  <div class="btnStrip">
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="windowControlStyle"
                        value="mac"
                        id="windowControlStyleMac"
                        data-var-type="string"
                        :checked="ob.windowControlStyle == 'mac'">
                      <label for="windowControlStyleMac">{{ ob.polyT('settings.advancedTab.appearance.macOS') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="windowControlStyle" value="win"
                        id="windowControlStyleWindows"
                        data-var-type="string"
                        :checked="ob.windowControlStyle == 'win'">
                      <label for="windowControlStyleWindows">{{ ob.polyT('settings.advancedTab.appearance.windowsOS') }}</label>
                    </div>
                  </div>
                </div>
              </div>
            </form>
          </div>
        </div>
      </div>
      <div class="contentBox padMd clrP clrBr clrSh3">
        <h2 class="h4 clrT">{{ ob.polyT('settings.advancedTab.transaction.sectionName') }}</h2>
        <hr class="clrBr" />
        <div class="tabFormWrapper clrS">
          <div class="js-transactionContainer">
            <form class="box padMdKids padStack clrP clrBr js-transaction">
              <div class="flexRow gutterH">
                <div class="col3">
                  <label>{{ ob.polyT('settings.advancedTab.transaction.saveMetadata') }}</label>
                  <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.helperSaveMetaData') }}</div>
                </div>
                <div class="col9">
                  <div class="btnStrip">
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="saveTransactionMetadata"
                        value="true"
                        id="saveTransactionMetadataOn"
                        data-var-type="boolean"
                        :checked="ob.saveTransactionMetadata">
                      <label for="saveTransactionMetadataOn">{{ ob.polyT('settings.on') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="saveTransactionMetadata"
                        value="false"
                        id="saveTransactionsMetadataOff"
                        data-var-type="boolean"
                        :checked="!ob.saveTransactionMetadata">
                      <label for="saveTransactionsMetadataOff">{{ ob.polyT('settings.off') }}</label>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flexRow gutterH js-feeSection">
                <div class="col3">
                  <label>{{ ob.polyT('settings.advancedTab.transaction.defaultFee') }}</label>
                  <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.helperDefaultFee') }}</div>
                </div>
                <div class="col9">
                  <FormError v-if="ob.errors['defaultTransactionFee']" :errors="ob.errors['defaultTransactionFee']" />
                  <div class="btnStrip">
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="defaultTransactionFee"
                        value="SUPER_ECONOMIC"
                        id="defaultTransactionFeeSuperLow"
                        data-var-type="string"
                        :checked="ob.defaultTransactionFee == 'SUPER_ECONOMIC'">
                      <label for="defaultTransactionFeeSuperLow">{{ ob.polyT('settings.advancedTab.transaction.superlow') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="defaultTransactionFee"
                        value="ECONOMIC"
                        id="defaultTransactionFeeLow"
                        data-var-type="string"
                        :checked="ob.defaultTransactionFee == 'ECONOMIC'">
                      <label for="defaultTransactionFeeLow">{{ ob.polyT('settings.advancedTab.transaction.low') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="defaultTransactionFee"
                        value="NORMAL"
                        id="defaultTransactionFeeMedium"
                        data-var-type="string"
                        :checked="ob.defaultTransactionFee == 'NORMAL'">
                      <label for="defaultTransactionFeeMedium">{{ ob.polyT('settings.advancedTab.transaction.medium') }}</label>
                    </div>
                    <div class="btnRadio clrBr">
                      <input type="radio"
                        data-persistence-location="local"
                        name="defaultTransactionFee"
                        value="PRIORITY"
                        id="defaultTransactionFeeHigh"
                        data-var-type="string"
                        :checked="ob.defaultTransactionFee == 'PRIORITY'">
                      <label for="defaultTransactionFeeHigh">{{ ob.polyT('settings.advancedTab.transaction.high') }}</label>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flexRow gutterH">
                <div class="col3">
                  <label>
                    {{ ob.polyT('settings.advancedTab.server.blockData') }}
                  </label>
                  <div class="clrT2 txSm">
                    {{ ob.polyT('settings.advancedTab.server.blockDataHelper') }}
                  </div>
                </div>
                <div class="col9">
                  <div class="flexVCent gutterH">
                    <ProcessingButton
                      :className="`btn clrP clrBr clrSh2 js-blockData ${ob.isGettingBlockData ? 'processing' : ''}`"
                      @click="clickBlockData"
                      :btnText="ob.polyT('settings.advancedTab.server.blockDataBtn')" />
                  </div>
                </div>
              </div>
            </form>
          </div>
        </div>
      </div>
      <div class="contentBox padMd clrP clrBr clrSh3">
        <h2 class="h4 clrT">{{ ob.polyT('settings.advancedTab.server.sectionName') }}</h2>
        <hr class="clrBr" />
        <div class="tabFormWrapper clrS">
          <div class="js-serverContainer">
            <form class="box padMdKids padStack clrP clrBr js-server">
              <div class="flexRow gutterH">
                <div class="col3">
                  <label>
                    {{ ob.polyT('settings.advancedTab.server.management') }}
                  </label>
                  <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.server.managementHelper') }}</div>
                </div>
                <div class="col9">
                  <a class="btn clrP clrBr clrSh2  flexNoShrink" @click="showConnectionManagement">{{ ob.polyT('settings.advancedTab.server.managementBtn') }}</a>
                </div>
              </div>
              <div class="flexRow gutterH">
                <div class="col3">
                  <label>
                    {{ ob.polyT('settings.advancedTab.server.peers') }}
                  </label>
                  <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.server.peersHelper') }}</div>
                </div>
                <div class="col9">
                  <a href="#connected-peers" class="btn clrP clrBr clrSh2 js-showConnectedPeers flexNoShrink">
                    {{ ob.polyT('settings.advancedTab.server.peersBtn') }}
                  </a>
                </div>
              </div>
              <div class="flexRow gutterH">
                <div class="col3">
                  <label>
                    {{ ob.polyT('settings.advancedTab.server.purge') }}
                  </label>
                  <div class="clrT2 txSm">
                    {{ ob.polyT('settings.advancedTab.server.purgeHelper') }}
                    <span class="toolTip toolTipTop" :data-tip="ob.polyT('settings.advancedTab.server.purgeToolTip')">
                      <i class="ion-help-circled"></i>
                    </span>
                  </div>
                </div>
                <div class="col9">
                  <div class="flexVCent gutterH">
                    <ProcessingButton
                      :className="`btn clrP clrBr clrSh2 js-purge ${ob.isPurging ? 'processing' : ''}`"
                      @click="clickPurge"
                      :btnText="ob.polyT('settings.advancedTab.server.purgeBtn')" />
                    <div class="js-purgeComplete hide">
                      <i class="h4 clrTEmph1">{{ ob.polyT('settings.advancedTab.server.purgeComplete') }}</i>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flexRow gutterH js-backupWalletSection">
                <div class="col3">
                  <label>
                    {{ ob.polyT('settings.advancedTab.server.backupWalletLbl') }}
                  </label>
                  <div class="clrT2 txSm">
                    {{ ob.polyT('settings.advancedTab.server.backupWalletHelper') }}
                  </div>
                </div>
                <div class="col9 js-walletSeedContainer">
                </div>
              </div>
            </form>
          </div>
        </div>
      </div>
      <template v-if="ob.bundled">
        <div class="contentBox padMd clrP clrBr clrSh3">
          <h2 class="h4 clrT">{{ ob.polyT('settings.advancedTab.integrations.sectionName') }}</h2>
          <hr class="clrBr" />
          <div class="tabFormWrapper clrS">
            <div class="">
              <form class="box padMdKids padStack clrP clrBr">
                <div class="flexRow gutterH">
                  <div class="col3">
                    <label>
                      {{ ob.polyT('settings.advancedTab.integrations.sharing') }}
                    </label>
                    <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.integrations.sharingHelper') }}</div>
                  </div>
                  <div class="col9 js-metricsStatusWrapper"></div>
                </div>
              </form>
            </div>
          </div>
        </div>
      </template>
      <div class="contentBox js-contentBoxEmailIntegration padMd clrP clrBr clrSh3">
        <h2 class="h4 clrT">{{ ob.polyT('settings.advancedTab.smtp.sectionName') }}</h2>
        <hr class="clrBr" />
        <div class="tabFormWrapper clrS">
          <div class="js-smtpSettingsContainer"></div>
        </div>
      </div>
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHRight">
          <ProcessingButton
            className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph js-save"
            @click="save"
            :btnText="ob.polyT('settings.btnSave')" />
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import { ipc } from '../../../../utils/ipcRenderer.js';
import app from '../../../../../backbone/app.js';
import { openSimpleMessage } from '../../SimpleMessage';
import Dialog from '../../../modals/Dialog';
import { endAjaxEvent, recordEvent, startAjaxEvent } from '../../../../../backbone/utils/metrics.js';
import loadTemplate from '../../../../../backbone/utils/loadTemplate.js';
import WalletSeed from './WalletSeed';
import SmtpSettings from './SmtpSettings';
import MetricsStatus from './MetricsStatus';


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
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: {
          ...(this.settings.validationError || {}),
          ...(this.localSettings.validationError || {}),
        },
        isPurging: this.purge && this.purge.state() === 'pending',
        isGettingBlockData: this.blockData && this.blockData.state() === 'pending',
        bundled,
        ...this._settings,
        ...this._localSettings,
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
      this.listenTo(this.settings, 'sync',
        (md, resp, opts) => app.settings.set(this.settings.toJSON(opts.attrs)));

      this.localSettings = app.localSettings.clone();

      // Sync our clone with any changes made to the global local settings model.
      this.listenTo(app.localSettings, 'someChange',
        (md, opts) => this.localSettings.set(opts.setAttrs));

      // Sync the global local settings model with any changes we save via our clone.
      this.listenTo(this.localSettings, 'sync',
        (md, resp, opts) => app.localSettings.set(this.localSettings.toJSON(opts.attrs)));
    },

    onClickShowSeed () {
      if (this.walletSeedFetch && this.walletSeedFetch.state() === 'pending') {
        return this.walletSeedFetch;
      }

      if (this.walletSeed) this.walletSeed.setState({ isFetching: true });

      recordEvent('Settings_Advanced_ShowSeed');

      this.walletSeedFetch = $.get(app.getServerUrl('wallet/mnemonic')).done((data) => {
        this.mnemonic = data.mnemonic;
        if (this.walletSeed) {
          this.walletSeed.setState({ seed: data.mnemonic });
        }
      }).always(() => {
        if (this.walletSeed) this.walletSeed.setState({ isFetching: false });
      })
        .fail(xhr => {
          openSimpleMessage(
            app.polyglot.t('settings.advancedTab.server.unableToFetchSeedTitle'),
            xhr.responseJSON && xhr.responseJSON.reason || ''
          );
        });

      return this.walletSeedFetch;
    },

    showConnectionManagement () {
      recordEvent('Settings_Advanced_ConnectionManagement');
      app.connectionManagmentModal.open();
    },

    getFormDataEx (subset = this.$formFields) {
      return this.getFormData(subset);
    },

    clickPurge () {
      recordEvent('Settings_PurgeCache');
      this.purgeCache();
    },

    /**
     * Call to the server to remove cached files that are being shared on IPFS.
     * This call should not be aborted when the view is removed, it's critical the user is informed if
     * the call fails, even if they have navigated away from the view.
     */
    purgeCache () {
      $('.js-purge').addClass('processing');
      $('.js-purgeComplete').addClass('hide');

      this.purge = $.post(app.getServerUrl('ob/purgecache'))
        .always(() => {
          $('.js-purge').removeClass('processing');
        })
        .fail((xhr) => {
          const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
          openSimpleMessage(
            app.polyglot.t('settings.advancedTab.server.purgeError'),
            failReason);
        })
        .done(() => {
          $('.js-purgeComplete').removeClass('hide');
        });
    },

    clickBlockData () {
      recordEvent('Settings_Advanced_ShowBlockData');
      this.showBlockData();
    },

    /**
     * Calls the server to retrieve and display information about the block the transactions are on
     */
    showBlockData () {
      $('.js-blockData').addClass('processing');

      this.blockData = $.get(app.getServerUrl('wallet/status'))
        .always(() => {
          $('.js-blockData').removeClass('processing');
        })
        .fail((xhr) => {
          const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
          openSimpleMessage(
            app.polyglot.t('settings.advancedTab.server.blockDataError'),
            failReason);
        })
        .done((data) => {
          const buttons = [{
            text: app.polyglot.t('settings.advancedTab.server.blockDataCopy'),
            fragment: 'copyBlockData',
          }];
          const message = Object.keys(data).map(coin => {
            // If the block isn't available, a long string of zeroes is returned.
            const hash = !/^0*$/.test(data[coin].bestHash) ? data[coin].bestHash :
              app.polyglot.t('settings.advancedTab.server.blockHashUnknown');
            const hashTxt = app.polyglot.t('settings.advancedTab.server.blockBestHash', { hash });
            const height = data[coin].height ||
              app.polyglot.t('settings.advancedTab.server.blockHeightUnknown');
            const heightTxt = app.polyglot.t('settings.advancedTab.server.blockHeight', { height });
            return {
              htmlString: `<p><b>${coin}</b><br>${hashTxt}<br>${heightTxt}</p>`,
              textString: `${coin}\n${hashTxt}\n${heightTxt}`,
            };
          });
          const blockDataDialog = new Dialog({
            title: app.polyglot.t('settings.advancedTab.server.blockDataTitle'),
            message: message.map(msg => msg.htmlString).join(''),
            messageClass: 'tx6',
            buttons,
            showCloseButton: true,
            removeOnClose: true,
          }).render().open();
          this.listenTo(blockDataDialog, 'click-copyBlockData', () => {
            ipc.send('controller.system.writeToClipboard', message.map(msg => msg.textString).join('\n\n'));
          });
        });
    },

    save () {
      this.localSettings.set(this.getFormDataEx(this.$localFields));
      this.localSettings.set({}, { validate: true });

      this.smtpSettings.setModelData();
      const serverFormData = {
        ...this.getFormDataEx(),
        smtpSettings: this.smtpSettings.model.toJSON(),
      };
      this.settings.set(serverFormData, { validate: true });

      if (!this.localSettings.validationError && !this.settings.validationError) {
        const msg = {
          msg: app.polyglot.t('settings.advancedTab.statusSaving'),
          type: 'message',
        };

        const statusMessage = app.statusBar.pushMessage({
          ...msg,
          duration: 9999999999999999,
        });

        startAjaxEvent('Settings_Advanced_Save');

        // let's save and monitor both save processes
        const localSave = this.localSettings.save();
        const serverSave = this.settings.save(serverFormData, {
          attrs: serverFormData,
          type: 'PUT',
        });

        $.when(localSave, serverSave)
          .done(() => {
            // both succeeded!
            statusMessage.update({
              msg: app.polyglot.t('settings.advancedTab.statusSaveComplete'),
              type: 'confirmed',
            });
            endAjaxEvent('Settings_Advanced_Save');
          })
          .fail((...args) => {
            // One has failed, the other may have also failed or may
            // fail or may succeed. It doesn't matter, for our purposed one
            // failure is enough for us to consider the "save" to have failed
            const errMsg = args[0] && args[0].responseJSON &&
              args[0].responseJSON.reason || '';

            openSimpleMessage(app.polyglot.t('settings.advancedTab.saveErrorAlertTitle'), errMsg);

            statusMessage.update({
              msg: app.polyglot.t('settings.advancedTab.statusSaveFailed'),
              type: 'warning',
            });
            endAjaxEvent('Settings_Advanced_Save', {
              errors: errMsg,
            });
          })
          .always(() => {
            $('.js-save').removeClass('processing');
            setTimeout(() => statusMessage.remove(), 3000);
          });
      }

      this.render();

      if (!this.localSettings.validationError && !this.settings.validationError) {
        $('.js-save').addClass('processing');
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

    get $smtpSettingsFields () {
      const selector = `.js-smtpSettingsForm select[name], .js-smtpSettingsForm input[name],
      .js-smtpSettingsForm textarea[name]:not([class*="trumbowyg"]),
      .js-smtpSettingsForm div[contenteditable][name]`;

      return $(selector);
    },

    render () {
      super.render();
      const bundled = ipc.sendSync('controller.system.getGlobal', 'isBundledApp');
      loadTemplate('modals/settings/advanced/advanced.html', (t) => {
        this.$el.html(t({
          errors: {
            ...(this.settings.validationError || {}),
            ...(this.localSettings.validationError || {}),
          },
          isPurging: this.purge && this.purge.state() === 'pending',
          isGettingBlockData: this.blockData && this.blockData.state() === 'pending',
          bundled,
          ...this.settings.toJSON(),
          ...this.localSettings.toJSON(),
        }));

        const formFieldsSelector = `
        .contentBox:not(.js-contentBoxEmailIntegration) select[name],
        .contentBox:not(.js-contentBoxEmailIntegration) input[name],
        .contentBox:not(.js-contentBoxEmailIntegration) textarea[name]
      `;
        this.$formFields = $(formFieldsSelector)
          .not('[data-persistence-location="local"]');
        this.$localFields = $('[data-persistence-location="local"]');

        if (this.walletSeed) this.walletSeed.remove();
        this.walletSeed = this.createChild(WalletSeed, {
          initialState: {
            seed: this.mnemonic || '',
            isFetching: this.walletSeedFetch && this.walletSeedFetch.state() === 'pending',
          },
        });
        this.listenTo(this.walletSeed, 'clickShowSeed', this.onClickShowSeed);
        $('.js-walletSeedContainer').append(this.walletSeed.render().el);

        if (this.smtpSettings) this.smtpSettings.remove();
        this.smtpSettings = this.createChild(SmtpSettings, {
          model: this.settings.get('smtpSettings'),
        });
        $('.js-smtpSettingsContainer').html(this.smtpSettings.render().el);

        if (this.metricsStatus) this.metricsStatus.remove();
        if (bundled) {
          this.metricsStatus = this.createChild(MetricsStatus);
          $('.js-metricsStatusWrapper').append(this.metricsStatus.render().el);
        }
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
