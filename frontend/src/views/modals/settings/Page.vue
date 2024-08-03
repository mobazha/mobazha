<template>
  <div class="settingsPage">
    <div class="gutterVMd2">
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHCent">
          <h2 class="h3 clrT">{{ ob.polyT('settings.pageTab.sectionName') }}</h2>
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
                <label for="settingsName" class="required">{{ ob.polyT('settings.name') }}</label>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['name']" :errors="ob.errors['name']" />
                <input type="text" class="clrBr clrSh2" v-model="formData.name" id="settingsName" :placeholder="ob.polyT('settings.pageTab.placeholderName')" />
              </div>
            </div>
            <div v-if="false" class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsHandle">{{ ob.polyT('settings.handle') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperHandle') }}</div>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['handle']" :errors="ob.errors['handle']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  v-model="formData.handle"
                  id="settingsHandle"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHandle')"
                />
                <div class="clrT2 txSm padSm">
                  {{ ob.polyT('settings.pageTab.helperHandleRegister', { helperHandleRegisterLink: `<a href="https://onename.com" class="clrTEm"
                    >${ob.polyT('settings.pageTab.helperHandleRegisterLink')}</a
                  >` }) }}
                </div>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsShortDescription">{{ ob.polyT('settings.shortDescription') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperShortDescription') }}</div>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['shortDescription']" :errors="ob.errors['shortDescription']" />
                <textarea
                  rows="3"
                  maxlength="160"
                  v-model="formData.shortDescription"
                  id="settingsShortDescription"
                  class="clrBr clrSh2"
                  :placeholder="ob.polyT('settings.pageTab.placeholderShortDescription')"
                  >{{ ob.shortDescription }}</textarea
                >
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsLocation">{{ ob.polyT('settings.pageTab.locationLabel') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperLocation') }}</div>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['location']" :errors="ob.errors['location']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  v-model="formData.location"
                  id="settingsLocation"
                  :placeholder="ob.polyT('settings.pageTab.placeholderLocation')"
                  :maxlength="ob.max.locationLength"
                />
              </div>
            </div>

            <div class="flexRow gutterH">
              <div class="col3">
                <label>{{ ob.polyT('settings.avatar') }}</label>
                <div class="clrT2 txSm">
                  {{
                    ob.polyT('settings.loadAvatarHelp', {
                      minWidth: ob.avatarMinWidth,
                      minHeight: ob.avatarMinHeight,
                    })
                  }}
                </div>
              </div>
              <div class="col9 contentBox clrBr clrP clrSh2 padLg">
                <div id="avatarCropper" ref="avatarCropper" class="flexRow gutterH">
                  <div ref="avatarPreview" class="contentBox avatarPreview clrP clrBr2 clrSh1 flexNoShrink js-avatarPreview"></div>
                  <div class="flexNoShrink">
                    <div class="flexColRows gutterV">
                      <div>
                        <div class="flex gutterH">
                          <a
                            :class="`iconBtn ion-reply flexExpand clrP clrBr clrSh2 js-avatarLeft ${!hasAvatarLoaded ? 'disabled' : ''}`"
                            @click="avatarLeftClick"
                          ></a>
                          <a
                            :class="`iconBtn ion-forward flexExpand clrP clrBr clrSh2 js-avatarRight ${!hasAvatarLoaded ? 'disabled' : ''}`"
                            @click="avatarRightClick"
                          ></a>
                        </div>
                      </div>
                      <div class="posR">
                        <input type="range" :class="`cropit-image-zoom-input js-avatarZoom clrP ${!hasAvatarLoaded ? 'disabled' : ''}`" />
                      </div>
                      <div>
                        <input type="file" id="avatarInput" ref="avatarInput" class="cropit-image-input hide" />
                        <label for="avatarInput" class="btn clrP clrBr clrSh2">
                          {{ ob.polyT('settings.loadAvatar') }}
                        </label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="false" class="flexRow gutterH TODO">
              <div class="col3">
                <label>{{ ob.polyT('settings.nsfwContent') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['nsfw']" :errors="ob.errors['nsfw']" />
                <div class="btnStrip">
                  <div class="btnRadio clrBr">
                    <input type="radio" name="nsfw" value="true" id="settingsNSFWInputTrue" data-var-type="boolean" :checked="ob.NSFW" />
                    <label for="settingsNSFWInputTrue">{{ ob.polyT('settings.yes') }}</label>
                  </div>
                  <div class="btnRadio clrBr">
                    <input type="radio" name="nsfw" value="false" id="settingsNSFWInputFalse" data-var-type="boolean" :checked="!ob.NSFW" />
                    <label for="settingsNSFWInputFalse">{{ ob.polyT('settings.no') }}</label>
                  </div>
                </div>
              </div>
            </div>
          </form>
        </div>
      </div>

      <div class="contentBox padMd clrP clrBr clrSh3">
        <h2 class="h4 clrT">{{ ob.polyT('settings.pageTab.sectionAbout') }}</h2>
        <hr class="clrBr" />
        <div class="tabFormWrapper">
          <form class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col12">
                <FormError v-if="ob.errors['about']" :errors="ob.errors['about']" />
                <Tinymce
                  class="clrBr clrSh2"
                  id="settingsAbout"
                  v-model="formData.about"
                  :menubar="'file edit insert view'"
                  :toolbar="['bold italic link image hr bullist numlist formatselect fontselect fontsizeselect fullscreen']"
                  :height="350"
                  :placeholder="ob.polyT('settings.pageTab.placeholderAbout')"
                ></Tinymce>
                <div class="clrT2 txSm padSm">{{ ob.polyT('settings.pageTab.helperAbout') }}</div>
              </div>
            </div>
          </form>
        </div>
      </div>

      <div class="contentBox padMd clrP clrBr clrSh3">
        <h2 class="h4 clrT">{{ ob.polyT('settings.pageTab.sectionLinks') }}</h2>
        <hr class="clrBr" />
        <div class="tabFormWrapper">
          <form class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsWebsite">{{ ob.polyT('settings.website') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperAbout') }}</div>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['contactInfo.website']" :errors="ob.errors['contactInfo.website']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  v-model="formData.contactInfo.website"
                  id="settingsWebsite"
                  :placeholder="ob.polyT('settings.pageTab.placeholderWebsite')"
                />
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsEmail">{{ ob.polyT('settings.email') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperAbout') }}</div>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['contactInfo.email']" :errors="ob.errors['contactInfo.email']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  v-model="formData.contactInfo.email"
                  id="settingsEmail"
                  :placeholder="ob.polyT('settings.pageTab.placeholderEmail')"
                />
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3"></div>
              <div class="col6">
                <FormError v-if="ob.errors['contactInfo.socialAccounts']" :errors="ob.errors['contactInfo.socialAccounts']" />
              </div>
            </div>
            <div class="js-socialAccounts">
              <SocialAccounts
                ref="socialAccounts"
                :options="{ maxAccounts: profile.get('contactInfo').maxSocialAccounts }"
                :bb="
                  () => {
                    return { collection: profile.get('contactInfo').get('social') };
                  }
                "
              />
            </div>
          </form>
        </div>
      </div>

      <div class="contentBox padMd clrP clrBr clrSh3">
        <h2 class="h4 clrT">{{ ob.polyT('settings.pageTab.sectionTheme') }}</h2>
        <hr class="clrBr" />
        <div class="tabFormWrapper">
          <form class="box padMdKids padStack">
            <div class="flexRow gutterH">
              <div class="col3">
                <label>{{ ob.polyT('settings.header') }}</label>
                <div class="clrT2 txSm">
                  {{
                    ob.polyT('settings.loadHeaderHelp', {
                      minWidth: ob.headerMinWidth,
                      minHeight: ob.headerMinHeight,
                    })
                  }}
                </div>
              </div>
              <div class="col9 row contentBox clrBr clrP clrSh2 pad">
                <div id="headerCropper" ref="headerCropper" class="flexColRows gutterV">
                  <div ref="headerPreview" class="contentBox headerPreview clrP clrBr js-headerPreview"></div>
                  <div>
                    <div class="flexRow gutterH">
                      <div>
                        <div class="flex gutterH">
                          <a :class="`iconBtn ion-reply flexExpand clrP clrBr clrSh2 ${!hasHeaderLoaded ? 'disabled' : ''}`" @click="headerLeftClick"></a>
                          <a :class="`iconBtn ion-forward flexExpand clrP clrBr clrSh2 ${!hasHeaderLoaded ? 'disabled' : ''}`" @click="headerRightClick"></a>
                        </div>
                      </div>
                      <div class="posR">
                        <input type="range" :class="`cropit-image-zoom-input js-headerZoom clrP ${!hasHeaderLoaded ? 'disabled' : ''}`" />
                      </div>
                      <div>
                        <input type="file" id="headerInput" ref="headerInput" class="cropit-image-input hide" />
                        <label for="headerInput" class="btn clrP clrBr clrSh2">
                          {{ ob.polyT('settings.loadHeader') }}
                        </label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div v-if="false" class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsPrimaryColor" class="required">{{ ob.polyT('settings.primaryColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperPrimaryColor') }}</div>
              </div>
              <div class="col1">
                <input
                  class="colorPicker clrBr"
                  id="primaryColorPicker"
                  v-model="formData.colors.primary"
                  data-hex-input-id="#settingsPrimaryColor"
                  type="color"
                  :placeholder="ob.polyT('settings.pageTab.placeholderPrimaryColor')"
                />
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.primary']" :errors="ob.errors['colors.primary']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  v-model="formData.colors.primary"
                  id="settingsPrimaryColor"
                  :placeholder="ob.polyT('settings.pageTab.placeholderPrimaryColor')"
                  data-color-picker-id="#primaryColorPicker"
                />
              </div>
            </div>
            <div v-if="false" class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsSecondaryColor" class="required">{{ ob.polyT('settings.secondaryColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperSecondaryColor') }}</div>
              </div>
              <div class="col1">
                <input
                  class="colorPicker clrBr"
                  id="secondaryColorPicker"
                  v-model="formData.colors.secondary"
                  data-hex-input-id="#settingsSecondaryColor"
                  type="color"
                  :placeholder="ob.polyT('settings.pageTab.placeholderSecondaryColor')"
                />
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.secondary']" :errors="ob.errors['colors.secondary']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  id="settingsSecondaryColor"
                  v-model="formData.colors.secondary"
                  :placeholder="ob.polyT('settings.pageTab.placeholderSecondaryColor')"
                  data-color-picker-id="#secondaryColorPicker"
                />
              </div>
            </div>
            <div v-if="false" class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsTextColor" class="required">{{ ob.polyT('settings.textColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperTextColor') }}</div>
              </div>
              <div class="col1">
                <input
                  class="colorPicker clrBr"
                  id="textColorPicker"
                  v-model="formData.colors.text"
                  data-hex-input-id="#settingsTextColor"
                  type="color"
                  :placeholder="ob.polyT('settings.pageTab.placeholderTextColor')"
                />
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.text']" :errors="ob.errors['colors.text']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  id="settingsTextColor"
                  v-model="formData.colors.text"
                  :placeholder="ob.polyT('settings.pageTab.placeholderTextColor')"
                  data-color-picker-id="#textColorPicker"
                />
              </div>
            </div>
            <div v-if="false" class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsHighlightColor" class="required">{{ ob.polyT('settings.highlightColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperHighlightColor') }}</div>
              </div>
              <div class="col1">
                <input
                  class="colorPicker clrBr"
                  v-model="formData.colors.highlight"
                  data-hex-input-id="#settingsHighlightColor"
                  type="color"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightTextColor')"
                />
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.highlight']" :errors="ob.errors['colors.highlight']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  id="settingsHighlightColor"
                  v-model="formData.colors.highlight"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightColor')"
                  data-color-picker-id="#highlightColorPicker"
                />
              </div>
            </div>
            <div v-if="false" class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsHighlightTextColor" class="required">{{ ob.polyT('settings.highlightTextColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperHighlightTextColor') }}</div>
              </div>
              <div class="col1">
                <input
                  class="colorPicker clrBr"
                  id="highlightTextColorPicker"
                  v-model="formData.colors.highlightText"
                  data-hex-input-id="#settingsHighlightTextColor"
                  type="color"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightTextColor')"
                />
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['color.highlightText']" :errors="ob.errors['color.highlightText']" />
                <input
                  type="text"
                  class="clrBr clrSh2"
                  id="settingsHighlightTextColor"
                  v-model="formData.colors.highlightText"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightTextColor')"
                  data-color-picker-id="#highlightTextColorPicker"
                />
              </div>
            </div>
          </form>
        </div>
      </div>

      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHRight">
          <ProcessingButton
            :className="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph js-save ${isSaving ? 'processing' : ''}`"
            @click="save"
            :btnText="ob.polyT('settings.btnSave')"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import app from '../../../../backbone/app';
import $ from 'jquery';
import '../../../../backbone/lib/whenAll.jquery';
import { myAjax } from '../../../api/api';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import 'cropit';

import SocialAccounts from './SocialAccounts.vue';
import Tinymce from '../../../components/Tinymce/index.vue';

export default {
  components: {
    SocialAccounts,
    Tinymce,
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
      isSaving: false,
      hasAvatarLoaded: false,
      hasHeaderLoaded: false,

      avatarMinWidth: 280,
      avatarMinHeight: 280,
      headerMinWidth: 2450,
      headerMinHeight: 700,

      formData: {
        name: '',
        shortDescription: '',
        location: '',
        about: '',
        contactInfo: {
          website: '',
          email: '',
        },
        // colors: {},
      },
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
        errors: this.profile.validationError || {},
        ...this.profile.toJSON(),
        max: this.profile.max,
        avatarMinHeight: this.avatarMinHeight,
        avatarMinWidth: this.avatarMinWidth,
        headerMinHeight: this.headerMinHeight,
        headerMinWidth: this.headerMinWidth,
      };
    },
  },
  methods: {
    loadData(options = {}) {
      this.baseInit(options);

      this.profile = app.profile.clone();

      this.formData = {
        name: this.profile.get('name'),
        shortDescription: this.profile.get('shortDescription'),
        location: this.profile.get('location'),
        about: this.profile.get('about'),
        contactInfo: {
          website: this.profile.get('contactInfo').get('website'),
          email: this.profile.get('contactInfo').get('email'),
        },
      };

      // Sync our clone with any changes made to the global profile.
      this.listenTo(app.profile, 'someChange', (md, opts) => this.profile.set(opts.setAttrs));

      // Sync the global profile with any changes we save via our clone.
      this.listenTo(this.profile, 'sync', () => {
        app.profile.set(this.profile.toJSON());
      });
    },

    avatarRotate(direction) {
      if (this.avatarCropper.cropit('imageSrc')) {
        this.avatarCropper.cropit(direction > 0 ? 'rotateCW' : 'rotateCCW');
        this.avatarChanged = true;
      }
    },

    avatarLeftClick() {
      this.avatarRotate(-1);
    },

    avatarRightClick() {
      this.avatarRotate(1);
    },

    headerRotate(direction) {
      if (this.headerCropper.cropit('imageSrc')) {
        this.headerCropper.cropit(direction > 0 ? 'rotateCW' : 'rotateCCW');
        this.headerChanged = true;
      }
    },

    headerLeftClick() {
      this.headerRotate(-1);
    },

    headerRightClick() {
      this.headerRotate(1);
    },

    saveHeader() {
      const imageURI = this.headerCropper.cropit('export', {
        type: 'image/jpeg',
        quality: 1,
        originalSize: true,
      });
      const headerData = JSON.stringify({ header: imageURI.replace(/^data:image\/(png|jpeg|webp);base64,/, '') });
      return myAjax({
        type: 'POST',
        url: app.getServerUrl('ob/header'),
        contentType: 'application/json; charset=utf-8',
        data: headerData,
        dataType: 'json',
      });
    },

    saveAvatar() {
      const imageURI = this.avatarCropper.cropit('export', {
        type: 'image/jpeg',
        quality: 1,
        originalSize: true,
      });
      const avatarData = JSON.stringify({ avatar: imageURI.replace(/^data:image\/(png|jpeg|webp);base64,/, '') });
      return myAjax({
        type: 'POST',
        url: app.getServerUrl('ob/avatar'),
        contentType: 'application/json; charset=utf-8',
        data: avatarData,
        dataType: 'json',
      });
    },

    getFormData() {
      const formData = this.formData;

      while (formData.handle && formData.handle.startsWith('@')) {
        formData.handle = formData.handle.slice(1);
      }

      if (formData.colors) {
        Object.keys(formData.colors).forEach((colorField) => {
          if (!formData.colors[colorField].startsWith('#')) {
            formData.colors[colorField] = `#${formData.colors[colorField]}`;
          }
        });
      }

      return formData;
    },

    save() {
      const formData = this.getFormData();

      // set the model data for the social accounts
      this.$refs.socialAccounts.setCollectionData();

      this.profile.set(formData);

      const save = this.profile.save();
      let saveAvatar;
      let saveHeader;

      if (save) {
        if (this.avatarOffsetOnLoad !== this.avatarCropper.cropit('offset') || this.avatarZoomOnLoad !== this.avatarCropper.cropit('zoom')) {
          this.avatarChanged = true;
        }

        if (this.headerOffsetOnLoad !== this.headerCropper.cropit('offset') || this.headerZoomOnLoad !== this.headerCropper.cropit('zoom')) {
          this.headerChanged = true;
        }

        if (this.avatarChanged && this.avatarCropper.cropit('imageSrc')) {
          saveAvatar = this.saveAvatar();
          saveAvatar.done((avatarData) => {
            // set hash in profile to mirror the server copy
            this.profile.set('avatarHashes', avatarData);
            app.profile.set('avatarHashes', avatarData);
          });
        }

        if (this.headerChanged && this.headerCropper.cropit('imageSrc')) {
          saveHeader = this.saveHeader();
          saveHeader.done((headerData) => {
            // set hash in profile to mirror the server copy
            this.profile.set('headerHashes', headerData);
            app.profile.set('headerHashes', headerData);
          });
        }

        const msg = {
          msg: app.polyglot.t('settings.pageTab.statusSaving'),
          type: 'message',
        };

        const statusMessage = app.statusBar.pushMessage({
          ...msg,
          duration: 9999999999999999,
        });

        $.whenAll(save, saveAvatar, saveHeader)
          .done(() => {
            statusMessage.update({
              msg: app.polyglot.t('settings.pageTab.statusSaveComplete'),
              type: 'confirmed',
            });
          })
          .fail((args) => {
            const errMsg = (args && args[0] && args[0].responseJSON && args[0].responseJSON.reason) || '';

            openSimpleMessage(app.polyglot.t('settings.pageTab.saveErrorAlertTitle'), errMsg);

            statusMessage.update({
              msg: app.polyglot.t('settings.pageTab.statusSaveFailed'),
              type: 'warning',
            });
          })
          .always(() => {
            this.isSaving = false;

            setTimeout(() => statusMessage.remove(), 3000);
          });
      }

      if (save) {
        this.isSaving = true;
      } else {
        const $firstErr = $('.errorList:first');

        if ($firstErr.length) {
          $firstErr[0].scrollIntoViewIfNeeded();
        } else {
          this.$emit('unrecognizedModelError', this, [this.profile]);
        }
      }
    },

    render() {
      let avatarURI = false;
      let headerURI = false;

      // if this is a re-render, get the contents of the cropits
      if (this.avatarCropper) {
        avatarURI = this.avatarCropper.cropit('export', {
          type: 'image/jpeg',
          quality: 1,
          originalSize: true,
        });
      }

      if (this.headerCropper) {
        headerURI = this.headerCropper.cropit('export', {
          type: 'image/jpeg',
          quality: 1,
          originalSize: true,
        });
      }

      this.avatarCropper = $(this.$refs.avatarCropper);

      this.headerCropper = $(this.$refs.headerCropper);

      // if the avatar or header exist, don't count the first load as a change
      this.avatarLoadedOnRender = Boolean(avatarURI || this.profile.get('avatarHashes').get('original'));
      this.headerLoadedOnRender = Boolean(headerURI || this.profile.get('headerHashes').get('original'));

      setTimeout(() => {
        this.avatarCropper.cropit({
          $preview: $(this.$refs.avatarPreview),
          $fileInput: $(this.$refs.avatarInput),
          smallImage: 'stretch',
          allowDragNDrop: false,
          maxZoom: 2,
          onImageLoaded: () => {
            this.avatarOffsetOnLoad = this.avatarCropper.cropit('offset');
            this.avatarZoomOnLoad = this.avatarCropper.cropit('zoom');

            this.hasAvatarLoaded = true;

            this.avatarChanged = !this.avatarLoadedOnRender;
            this.avatarLoadedOnRender = false;
          },
          onFileReaderError: (data) => {
            console.log('file reader error');
            console.log(data);
          },
          onImageError: (errorObject) => {
            console.log(errorObject.code);
            console.log(errorObject.message);
          },
        });

        this.headerCropper.cropit({
          $preview: $(this.$refs.headerPreview),
          $fileInput: $(this.$refs.headerInput),
          smallImage: 'stretch',
          allowDragNDrop: false,
          maxZoom: 2,
          onImageLoaded: () => {
            this.headerOffsetOnLoad = this.headerCropper.cropit('offset');
            this.headerZoomOnLoad = this.headerCropper.cropit('zoom');

            this.hasHeaderLoaded = true;

            this.headerChanged = !this.headerLoadedOnRender;
            this.headerLoadedOnRender = false;
          },
          onFileReaderError: (data) => {
            console.log('file reader error');
            console.log(data);
          },
          onImageError: (errorObject) => {
            console.log(errorObject.code);
            console.log(errorObject.message);
          },
        });

        if (avatarURI) {
          this.avatarCropper.cropit('imageSrc', avatarURI);
        } else if (this.profile.get('avatarHashes').get('original')) {
          this.avatarCropper.cropit('imageSrc', app.getServerUrl(`ob/image/${this.profile.get('avatarHashes').get('original')}`));
        }

        if (headerURI) {
          this.headerCropper.cropit('imageSrc', headerURI);
        } else if (this.profile.get('headerHashes').get('original')) {
          this.headerCropper.cropit('imageSrc', app.getServerUrl(`ob/image/${this.profile.get('headerHashes').get('original')}`));
        }
      }, 0);

      return this;
    },
  },
};
</script>
<style lang="scss" scoped>
::v-deep(.tox-fullscreen) {
  top: 50px !important;
}
</style>
