<template>
  <div class="modal onboarding modalScrollPage modalMedium">
    <BaseModal>
      <template v-slot:component>
        <section :class="`contentBox padMd clrP clrBr clrSh3 ${`${ob.screen}Screen`}`">
          <div class="flexVCent gutterH posR">
            <div :class="ob.screen === 'intro' ? 'flexExpand' : ''">
              {{ ob.brandingBoxT() }}
            </div>
            <template v-if="ob.screen === 'intro'">
              <template v-if="ob.curConn && ob.curConn.server">
                  <div><span class="txB">{{ ob.polyT('onboarding.introScreen.connectionLbl') }}</span> {{ ob.curConn.server.get('name') || '' }}</div>
                  <div>
                    <button class="btn clrP " @click="onClickChangeServer">{{ ob.polyT('onboarding.introScreen.btnChange') }}</button>
                  </div>
              </template>
            </template>

            <template v-else-if="ob.screen === 'info'">
              <div class="flexExpand txB tx3">
                <div class="center">{{ ob.polyT('onboarding.infoScreen.heading') }}</div>
              </div>
              <div class="clrT2 txB">{{ ob.polyT('onboarding.pageOfPage', { startPage: 1, totalPages: 2 }) }}</div>
            </template>

            <template v-else-if="ob.screen === 'tos'">
              <div class="flexExpand txB tx3">
                <div class="center">{{ ob.polyT('onboarding.tosScreen.heading') }}</div>
              </div>
              <div class="clrT2 txB">{{ ob.polyT('onboarding.pageOfPage', { startPage: 2, totalPages: 2 }) }}</div>
            </template>
          </div>
          <hr class="clrBr" />
          <div class="mainContent">
            <template v-if="ob.screen === 'intro'">
              <div class="flexCent">
                <div>
                  <div class="txCtr rowSm">
                    <span class="txUp txB clrT2"><i>{{ ob.polyT('onboarding.introScreen.introLine') }}</i></span>
                  </div>
                  <div class="headline clrT txCtr rowHg">{{ ob.polyT('onboarding.introScreen.tagLine') }}</div>
                  <div class="txCtr">
                    <button class="btnGetStarted btnHg clrBAttGrad clrBrDec1 clrTOnEmph " @click="onClickGetStarted">{{
                      ob.polyT('onboarding.introScreen.btnGetStarted') }}<span class="ion-chevron-right margL"></span></button>
                  </div>
                </div>
              </div>
            </template>

            <template v-else-if="ob.screen === 'info'">
              <form class="padStack">
                <div class="row">
                  <label for="onboardingName" class="required">{{ ob.polyT('onboarding.infoScreen.nameLbl') }}</label>
                    <FormError v-if="ob.profileErrors['name']" :errors="ob.profileErrors['name']" />
                    <input type="text" class="clrBr clrSh2" v-model="formData.profile.name" id="onboardingName" :placeholder="ob.polyT('onboarding.infoScreen.placeholderName')">
                </div>
                <div class="row">
                  <div class="flexVBase">
                    <label for="onboardingShortDescription" class="flexExpand">{{ ob.polyT('onboarding.infoScreen.descriptionLbl') }}</label>
                    <div class="clrT2 tx6">{{ ob.polyT('onboarding.infoScreen.descriptionHelper', { count: ob.profileConstraints.shortDescriptionLength }) }}</div>
                  </div>
                    <FormError v-if="ob.profileErrors['shortDescription']" :errors="ob.profileErrors['shortDescription']" />
                    <textarea rows="3" :maxlength="ob.profileConstraints.shortDescriptionLength"
                      v-model="formData.profile.shortDescription"
                      id="onboardingShortDescription" class="clrBr clrSh2"
                      :placeholder="ob.polyT('onboarding.infoScreen.placeholderDescription')"></textarea>
                </div>
                <div class="row">
                  <label>{{ ob.polyT('onboarding.infoScreen.avatarLbl') }}</label>
                  <div class="border clrBr pad avatarCropperWrap">
                    <div ref="avatarCropper" class="flexRow flexVCent gutterH" id="avatarCropper">
                      <div ref="avatarPreview" class="contentBox avatarPreview clrP clrBr2 clrSh1 flexNoShrink"></div>
                      <div class="flexNoShrink">
                        <div class="flexColRows gutterVTn avatarCropControls">
                          <div>
                            <div class="flex gutterH">
                              <button class="iconBtn ion-reply flexExpand clrP clrBr clrSh2 avatarLeft" :disabled="!imageLoaded" @click="onAvatarLeftClick"></button>
                              <button class="iconBtn ion-forward flexExpand clrP clrBr clrSh2 avatarRight" :disabled="!imageLoaded" @click="onAvatarRightClick"></button>
                            </div>
                          </div>
                          <div class="posR">
                            <input type="range" class="cropit-image-zoom-input clrP" :disabled="!imageLoaded" value=0 />
                          </div>
                        </div>
                      </div>
                      <div>
                        <input type="file" ref="avatarInput" class="cropit-image-input invisible posA" tabindex="-1" />
                        <button class="btn clrP clrBr clrSh2 tx6 " @click="$refs.avatarInput.click()">
                          {{ ob.polyT('onboarding.infoScreen.changeAvatarLbl') }}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
                <div class="row">
                  <label for="onboardingCountry" class="required">{{ ob.polyT('onboarding.infoScreen.countryLbl') }}</label>
                    <FormError v-if="ob.settingsErrors['country']" :errors="ob.settingsErrors['country']" />
                    <Select2 id="onboardingCountry" v-model="formData.settings.country" class="clrSh2">
                      <template  v-for="(country, j) in ob.countryList" :key="country.dataName">
                        <option :value="country.dataName" :selected="country.dataName == ob.settings.country">{{ country.name }}</option>
                      </template>
                    </Select2>
                </div>
                <div class="row">
                  <label for="onboardingCurrency" class="required">{{ ob.polyT('onboarding.infoScreen.currencyLbl') }}</label>
                    <FormError v-if="ob.settingsErrors['currency']" :errors="ob.settingsErrors['currency']" />
                    <Select2 id="onboardingCurrency" v-model="formData.settings.localCurrency" class="clrSh2">
                      <template v-for="(currency, j) in ob.currencyList" :key="currency.code">
                        <option :value="currency.code" :selected="currency.code == ob.settings.localCurrency">{{ currency.nameWithCode }}</option>
                      </template>
                    </Select2>
                </div>
              </form>
            </template>

            <template v-else-if="ob.screen === 'tos'">
              <p v-for="(p, i) in tosBreakContent" :key="i">{{ p.trim() }}</p>
            </template>
          </div>
          <template v-if="ob.screen !== 'intro'">
            <hr class="clrBr row" />
            <div class="flexVCent">
              <div class="flexExpand">
                <button class="btn clrP " @click="onClickNavBack">Back</button>
              </div>
              <div>
                <template v-if="ob.screen !== 'tos'">
                  <button class="btn clrP " @click="onClickNavNext">Next</button>
                </template>

                <template v-else>
                    <ProcessingButton
                      :className="`btn clrP js-tosAgree ${ob.saveInProgress ? 'processing' : '' }`"
                      @click="onClickTosAgree"
                      :btnText="I Agree" />
                </template>
              </div>
            </div>
          </template>
        </section>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import 'cropit';
import app from '../../../../backbone/app';
import { myAjax } from '../../../api/api';
import { getCurrentConnection } from '../../../../backbone/utils/serverConnect';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
import { getCurrencies } from '../../../../backbone/data/currencies';
import { openSimpleMessage } from '../SimpleMessage';
import loadTemplate from '../../../../backbone/utils/loadTemplate';


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
      imageLoaded: false,

      formData: {
        settings: {
          country: '',
          localCurrency: '',
        }
      },
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
        brandingBoxT,
        ...state,
        curConn: getCurrentConnection(),
        profile: app.profile.toJSON(),
        profileErrors: app.profile.validationError || {},
        profileConstraints: app.profile.max,
        settings: app.settings.toJSON(),
        settingsErrors: app.settings.validationError || {},
        countryList: this.countryList,
        currencyList: this.currencyList,
      };
    },

    tosBreakContent() {
      // split the TOS on line breaks so that we could ouput in p elements let
      let tos=ob.polyT('onboarding.tosScreen.tos').replace('\r', '\n' );
      tos=tos.replace(/\n\s*\n/g, '--->break-here<---' );
      return tos.split('--->break-here<---');
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        dismissOnEscPress: false,
        showCloseButton: false,
        initialState: {
          screen: 'intro',
          saveInProgress: false,
          ...options.initialState,
        },
        ...options,
      };

      this.baseInit(opts);
      this.options = opts;
      this.screens = ['intro', 'info', 'tos'];
      this.lastAvatarImageRotate = 0;
      this.avatarChanged = false;
      this.countryList = getTranslatedCountries();
      this.currencyList = getCurrencies();
    },

    onClickChangeServer () {
      app.connectionManagmentModal.open();
    },

    onClickGetStarted () {
      this.setState({ screen: 'info' });
    },

    onClickNavBack () {
      const curScreen = this.getState().screen;
      const newScreen = this.screens[this.screens.indexOf(curScreen) - 1];

      if (curScreen === 'info') {
        this.setModelsFromForm();
      }

      this.setState({
        screen: newScreen,
      });
    },

    onClickNavNext () {
      const curScreen = this.getState().screen;
      const newScreen = this.screens[this.screens.indexOf(curScreen) + 1];

      if (curScreen === 'info') {
        this.setModelsFromForm();

        if (newScreen === 'tos') {
          app.profile.set({}, { validate: true });
          app.settings.set({}, { validate: true });

          if (app.settings.validationError || app.profile.validationError) {
            this.render();
            return;
          }
        }
      }

      this.setState({ screen: newScreen });
    },

    onAvatarLeftClick () {
      this.avatarRotate(-1);
    },

    onAvatarRightClick () {
      this.avatarRotate(1);
    },

    onClickTosAgree () {
      this.setState({ saveInProgress: true });

      const profileSave = app.profile.save({}, {
        type:
          Object.keys(app.profile.lastSyncedAttrs).length ?
            'PUT' : 'POST',
      });

      const settingsSave = app.settings.save({}, {
        type:
          Object.keys(app.settings.lastSyncedAttrs).length ?
            'PUT' : 'POST',
      });

      const saves = [profileSave, settingsSave];

      if (this.avatarChanged) {
        const avatarSave = this.saveAvatar()
          .done(avatarData => app.profile.set('avatarHashes', avatarData));
        saves.push(avatarSave);
      }

      $.when(...saves).done(() => {
        this.trigger('onboarding-complete');
      }).fail((jqXhr) => {
        let title;

        if (jqXhr === profileSave) {
          title = app.polyglot.t('onboarding.profileFailedSaveTitle');
        } else if (jqXhr === settingsSave) {
          title = app.polyglot.t('onboarding.settingsFailedSaveTitle');
        } else {
          title = app.polyglot.t('onboarding.settingsFailedSaveAvatar');
        }

        openSimpleMessage(title, jqXhr.responseJSON && jqXhr.responseJSON.reason || '');
      })
        .always(() => {
          this.setState({ saveInProgress: false });
        });
    },

    setModelsFromForm () {
      app.settings.set(this.formData.settings);
      app.profile.set(this.formData.profile);
    },

    saveAvatar () {
      if (!this.avatarExport) {
        throw new Error('Unable to save the avatar because the export ' +
          'data is not available');
      }

      const avatarData = JSON.stringify(
        { avatar: this.avatarExport.replace(/^data:image\/(png|jpeg|webp);base64,/, '') });

      return myAjax({
        type: 'POST',
        url: app.getServerUrl('ob/avatar'),
        contentType: 'application/json; charset=utf-8',
        data: avatarData,
        dataType: 'json',
      });
    },

    avatarRotate (direction) {
      if (this.$avatarCropper.cropit('imageSrc')) {
        this.$avatarCropper.cropit(direction > 0 ? 'rotateCW' : 'rotateCCW');

        // normalize so this.lastAvatarImageRotate is a positive number between 0 and 3
        this.lastAvatarImageRotate = (this.lastAvatarImageRotate + direction) % 4;
        if (this.lastAvatarImageRotate === -1) {
          this.lastAvatarImageRotate = 3;
        } else if (this.lastAvatarImageRotate === -2) {
          this.lastAvatarImageRotate = 2;
        } else if (this.lastAvatarImageRotate === -3) {
          this.lastAvatarImageRotate = 1;
        }
      }
    },

    render () {
      if (this.$avatarCropper) {
        this.lastAvatarZoom = this.$avatarCropper.cropit('zoom');
        this.lastAvatarImageSrc = this.$avatarCropper.cropit('imageSrc');
        this.avatarExport = this.$avatarCropper.cropit('export', {
          type: 'image/jpeg',
          quality: 1,
          originalSize: true,
        });
        this.$avatarCropper = null;
      }

      if (state.screen === 'info') {
        setTimeout(() => {
          this.$avatarCropper = this.refs.avatarCropper.cropit({
            $preview: this.$refs.avatarPreview,
            $fileInput: this.$refs.avatarInput,
            smallImage: 'stretch',
            allowDragNDrop: false,
            maxZoom: 2,
            onImageLoaded: () => {
              this.imageLoaded = true;
              this.$avatarCropper.cropit('zoom', this.lastAvatarZoom);

              for (let i = 0; i < this.lastAvatarImageRotate; i++) {
                this.$avatarCropper.cropit('rotateCW');
              }
            },
            onFileChange: () => {
              this.lastAvatarImageRotate = 0;
              this.lastAvatarImageSrc = '';
              this.lastAvatarZoom = 0;
              this.avatarChanged = true;
            },
            onFileReaderError: (data) => {
              console.log('file reader error');
              console.log(data);
            },
            onImageError: (errorObject) => {
              console.log(errorObject.code);
              console.log(errorObject.message);
            },
            imageState: {
              src: this.lastAvatarImageSrc || '',
            },
          });
        }, 0);
      }

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
