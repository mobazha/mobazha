<template>
  <div class="settingsPage">
    <div class="gutterVMd2">
      <div class="contentBox padMd clrP clrBr clrSh3">
        <div class="flexHCent">
          <h2 class="h3 clrT">{{ ob.polyT('settings.pageTab.sectionPage') }}</h2>
          <ProcessingButton
            className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn js-save"
            @click="save" :btnText="ob.polyT('settings.btnSave')" />
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
                <input type="text" class="clrBr clrSh2" name="name" id="settingsName" :value="ob.name"
                  :placeholder="ob.polyT('settings.pageTab.placeholderName')">
              </div>
            </div>
            <div class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsHandle">{{ ob.polyT('settings.handle') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperHandle') }}</div>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['handle']" :errors="ob.errors['handle']" />
                <input type="text" class="clrBr clrSh2" name="handle" id="settingsHandle" :value="ob.handle"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHandle')">
                <div class="clrT2 txSm padSm">{{ ob.polyT('settings.pageTab.helperHandleRegister', {
                  helperHandleRegisterLink: `<a href="https://onename.com"
                    class="clrTEm">${ob.polyT('settings.pageTab.helperHandleRegisterLink')}</a>`
                }) }}</div>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsShortDescription">{{ ob.polyT('settings.shortDescription') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperShortDescription') }}</div>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['shortDescription']" :errors="ob.errors['shortDescription']" />
                <textarea rows="3" maxlength="160" name="shortDescription" id="settingsShortDescription"
                  class="clrBr clrSh2"
                  :placeholder="ob.polyT('settings.pageTab.placeholderShortDescription')">{{ ob.shortDescription }}</textarea>
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsLocation">{{ ob.polyT('settings.pageTab.locationLabel') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperLocation') }}</div>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['location']" :errors="ob.errors['location']" />
                <input type="text" class="clrBr clrSh2" name="location" id="settingsLocation" :value="ob.location"
                  :placeholder="ob.polyT('settings.pageTab.placeholderLocation')" :maxlength="ob.max.locationLength">
              </div>
            </div>

            <div class="flexRow gutterH">
              <div class="col3">
                <label>{{ ob.polyT('settings.avatar') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.loadAvatarHelp', {
                  minWidth: ob.avatarMinWidth, minHeight:
                    ob.avatarMinHeight
                }) }}</div>
              </div>
              <div class="col9 contentBox clrBr clrP clrSh2 padLg">
                <div class="flexRow gutterH" id="avatarCropper">
                  <div class="contentBox avatarPreview clrP clrBr2 clrSh1 flexNoShrink js-avatarPreview"></div>
                  <div class="flexNoShrink">
                    <div class="flexColRows gutterV">
                      <div>
                        <div class="flex gutterH">
                          <a class="iconBtn ion-reply flexExpand clrP clrBr clrSh2 disabled" @click="avatarLeftClick"></a>
                          <a class="iconBtn ion-forward flexExpand clrP clrBr clrSh2 disabled" @click="avatarRightClick"></a>
                        </div>
                      </div>
                      <div class="posR">
                        <input type="range" class="cropit-image-zoom-input disabled js-avatarZoom clrP" />
                      </div>
                      <div>
                        <input type="file" id="avatarInput" class="cropit-image-input hide" />
                        <label for="avatarInput" class="btn clrP clrBr clrSh2">
                          {{ ob.polyT('settings.loadAvatar') }}
                        </label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="flexRow gutterH TODO">
              <div class="col3">
                <label>{{ ob.polyT('settings.nsfwContent') }}</label>
              </div>
              <div class="col9">
                <FormError v-if="ob.errors['nsfw']" :errors="ob.errors['nsfw']" />
                <div class="btnStrip">
                  <div class="btnRadio clrBr">
                    <input
                      type="radio"
                      name="nsfw"
                      value="true"
                      id="settingsNSFWInputTrue"
                      data-var-type="boolean"
                      :checked="ob.NSFW">
                    <label for="settingsNSFWInputTrue">{{ ob.polyT('settings.yes') }}</label>
                  </div>
                  <div class="btnRadio clrBr">
                    <input
                      type="radio"
                      name="nsfw"
                      value="false"
                      id="settingsNSFWInputFalse"
                      data-var-type="boolean"
                      :checked="!ob.NSFW">
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
                <div contenteditable class="clrBr clrSh2" name="about" id="settingsAbout"
                  :placeholder="ob.polyT('settings.pageTab.placeholderAbout')">{{ ob.about }}</div>
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
                <input type="text" class="clrBr clrSh2" name="contactInfo.website" id="settingsWebsite"
                  :value="ob.contactInfo.website" :placeholder="ob.polyT('settings.pageTab.placeholderWebsite')">
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3">
                <label for="settingsEmail">{{ ob.polyT('settings.email') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperAbout') }}</div>
              </div>
              <div class="col6">
                <FormError v-if="ob.errors['contactInfo.email']" :errors="ob.errors['contactInfo.email']" />
                <input type="text" class="clrBr clrSh2" name="contactInfo.email" id="settingsEmail"
                  :value="ob.contactInfo.email" :placeholder="ob.polyT('settings.pageTab.placeholderEmail')">
              </div>
            </div>
            <div class="flexRow gutterH">
              <div class="col3"></div>
              <div class="col6">
                <FormError v-if="ob.errors['contactInfo.socialAccounts']"
                  :errors="ob.errors['contactInfo.socialAccounts']" />
              </div>
            </div>
            <div class="js-socialAccounts"></div>
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
                <div class="clrT2 txSm">{{ ob.polyT('settings.loadHeaderHelp', {
                  minWidth: ob.headerMinWidth, minHeight:
                    ob.headerMinHeight
                }) }}</div>
              </div>
              <div class="col9 row contentBox clrBr clrP clrSh2 pad">
                <div class="flexColRows gutterV" id="headerCropper">
                  <div class="contentBox headerPreview clrP clrBr js-headerPreview"></div>
                  <div>
                    <div class="flexRow gutterH">
                      <div>
                        <div class="flex gutterH">
                          <a class="iconBtn ion-reply flexExpand clrP clrBr clrSh2 disabled" @click="headerLeftClick"></a>
                          <a class="iconBtn ion-forward flexExpand clrP clrBr clrSh2 disabled" @click="headerRightClick"></a>
                        </div>
                      </div>
                      <div class="posR">
                        <input type="range" class="cropit-image-zoom-input disabled js-headerZoom clrP" />
                      </div>
                      <div>
                        <input type="file" id="headerInput" class="cropit-image-input hide" />
                        <label for="headerInput" class="btn clrP clrBr clrSh2">
                          {{ ob.polyT('settings.loadHeader') }}
                        </label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsPrimaryColor" class="required">{{ ob.polyT('settings.primaryColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperPrimaryColor') }}</div>
              </div>
              <div class="col1">
                <input class="colorPicker clrBr" @change="handleColorChosen" id="primaryColorPicker"
                  data-hex-input-id="#settingsPrimaryColor" type="color" :value="ob.colors.primary"
                  :placeholder="ob.polyT('settings.pageTab.placeholderPrimaryColor')">
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.primary']" :errors="ob.errors['colors.primary']" />
                <input type="text" class="clrBr clrSh2" @change="handleColorCodeEntered" name="colors.primary"
                  id="settingsPrimaryColor" :value="ob.colors.primary"
                  :placeholder="ob.polyT('settings.pageTab.placeholderPrimaryColor')"
                  data-color-picker-id="#primaryColorPicker">
              </div>
            </div>
            <div class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsSecondaryColor" class="required">{{ ob.polyT('settings.secondaryColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperSecondaryColor') }}</div>
              </div>
              <div class="col1">
                <input class="colorPicker clrBr" @change="handleColorChosen" id="secondaryColorPicker"
                  data-hex-input-id="#settingsSecondaryColor" type="color" :value="ob.colors.secondary"
                  :placeholder="ob.polyT('settings.pageTab.placeholderSecondaryColor')">
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.secondary']" :errors="ob.errors['colors.secondary']" />
                <input type="text" class="clrBr clrSh2" @change="handleColorCodeEntered" name="colors.secondary"
                  id="settingsSecondaryColor" :value="ob.colors.secondary"
                  :placeholder="ob.polyT('settings.pageTab.placeholderSecondaryColor')"
                  data-color-picker-id="#secondaryColorPicker">
              </div>
            </div>
            <div class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsTextColor" class="required">{{ ob.polyT('settings.textColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperTextColor') }}</div>
              </div>
              <div class="col1">
                <input class="colorPicker clrBr " @change="handleColorChosen" id="textColorPicker"
                  data-hex-input-id="#settingsTextColor" type="color" :value="ob.colors.text"
                  :placeholder="ob.polyT('settings.pageTab.placeholderTextColor')">
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.text']" :errors="ob.errors['colors.text']" />
                <input type="text" class="clrBr clrSh2 " @change="handleColorCodeEntered" name="colors.text"
                  id="settingsTextColor" :value="ob.colors.text"
                  :placeholder="ob.polyT('settings.pageTab.placeholderTextColor')"
                  data-color-picker-id="#textColorPicker">
              </div>
            </div>
            <div class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsHighlightColor" class="required">{{ ob.polyT('settings.highlightColor') }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperHighlightColor') }}</div>
              </div>
              <div class="col1">
                <input class="colorPicker clrBr " @change="handleColorChosen" id="highlightColorPicker"
                  data-hex-input-id="#settingsHighlightColor" type="color" :value="ob.colors.highlight"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightTextColor')">
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['colors.highlight']" :errors="ob.errors['colors.highlight']" />
                <input type="text" class="clrBr clrSh2 " @change="handleColorCodeEntered" name="colors.highlight"
                  id="settingsHighlightColor" :value="ob.colors.highlight"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightColor')"
                  data-color-picker-id="#highlightColorPicker">
              </div>
            </div>
            <div class="flexRow gutterH TODO">
              <div class="col3">
                <label for="settingsHighlightTextColor" class="required">{{ ob.polyT('settings.highlightTextColor')
                }}</label>
                <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperHighlightTextColor') }}</div>
              </div>
              <div class="col1">
                <input class="colorPicker clrBr " @change="handleColorChosen" id="highlightTextColorPicker"
                  data-hex-input-id="#settingsHighlightTextColor" type="color" :value="ob.colors.highlightText"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightTextColor')">
              </div>
              <div class="col2">
                <FormError v-if="ob.errors['color.highlightText']" :errors="ob.errors['color.highlightText']" />
                <input type="text" class="clrBr clrSh2 " @change="handleColorCodeEntered" name="colors.highlightText"
                  id="settingsHighlightTextColor" :value="ob.colors.highlightText"
                  :placeholder="ob.polyT('settings.pageTab.placeholderHighlightTextColor')"
                  data-color-picker-id="#highlightTextColorPicker">
              </div>
            </div>
          </form>
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
/* eslint-disable class-methods-use-this */
import app from '../../../../backbone/app';
import $ from 'jquery';
import '../../../../backbone/lib/whenAll.jquery';
import { openSimpleMessage } from '../SimpleMessage';
import 'cropit';
import { installRichEditor } from '../../../../backbone/utils/lib/trumbowyg';
import SocialAccounts from './SocialAccounts';


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
      avatarMinWidth: 280,
      avatarMinHeight: 280,
      headerMinWidth: 2450,
      headerMinHeight: 700,
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
        errors: this.profile.validationError || {},
        ...this._profile,
        max: this.profile.max,
        avatarMinHeight: this.avatarMinHeight,
        avatarMinWidth: this.avatarMinWidth,
        headerMinHeight: this.headerMinHeight,
        headerMinWidth: this.headerMinWidth,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this.profile = app.profile.clone();

      // Sync our clone with any changes made to the global profile.
      this.listenTo(app.profile, 'someChange', (md, opts) => this.profile.set(opts.setAttrs));

      // Sync the global profile with any changes we save via our clone.
      this.listenTo(this.profile, 'sync', () => {
        app.profile.set(this.profile.toJSON());
      });

      this.socialAccounts = this.createChild(SocialAccounts, {
        collection: this.profile.get('contactInfo').get('social'),
        maxAccounts: this.profile.get('contactInfo').maxSocialAccounts,
      });
    },

    /** Handles when a hex color code is entered by updating color picker. */
    handleColorCodeEntered (event) {
      const colorPickerId = $(event.target).data('color-picker-id');
      const $colorPicker = this.getCachedEl(colorPickerId);
      const newHexColorCode = event.target.value;

      // If the text passes a basic RegExp for a valid 6 digit hex value,
      // update the color picker's color.
      if (/^#([0-9a-f]{6})$/i.test(newHexColorCode)) {
        $colorPicker.val(newHexColorCode);
      }
    },

    /** Handles when a color is chosen from the color picker by updating hex color code text. */
    handleColorChosen (event) {
      const hexInputId = $(event.target).data('hex-input-id');
      const $hexInput = this.getCachedEl(hexInputId);
      const newColor = event.target.value;

      $hexInput.val(newColor);
    },

    avatarRotate (direction) {
      if (this.avatarCropper.cropit('imageSrc')) {
        this.avatarCropper.cropit(direction > 0 ? 'rotateCW' : 'rotateCCW');
        this.avatarChanged = true;
      }
    },

    avatarLeftClick () {
      this.avatarRotate(-1);
    },

    avatarRightClick () {
      this.avatarRotate(1);
    },

    headerRotate (direction) {
      if (this.headerCropper.cropit('imageSrc')) {
        this.headerCropper.cropit(direction > 0 ? 'rotateCW' : 'rotateCCW');
        this.headerChanged = true;
      }
    },

    headerLeftClick () {
      this.headerRotate(-1);
    },

    headerRightClick () {
      this.headerRotate(1);
    },

    saveHeader () {
      const imageURI = this.headerCropper.cropit('export', {
        type: 'image/jpeg',
        quality: 1,
        originalSize: true,
      });
      const headerData = JSON.stringify(
        { header: imageURI.replace(/^data:image\/(png|jpeg|webp);base64,/, '') });
      return $.ajax({
        type: 'POST',
        url: app.getServerUrl('ob/header'),
        contentType: 'application/json; charset=utf-8',
        data: headerData,
        dataType: 'json',
      });
    },

    saveAvatar () {
      const imageURI = this.avatarCropper.cropit('export', {
        type: 'image/jpeg',
        quality: 1,
        originalSize: true,
      });
      const avatarData = JSON.stringify(
        { avatar: imageURI.replace(/^data:image\/(png|jpeg|webp);base64,/, '') });
      return $.ajax({
        type: 'POST',
        url: app.getServerUrl('ob/avatar'),
        contentType: 'application/json; charset=utf-8',
        data: avatarData,
        dataType: 'json',
      });
    },

    getFormDataEx () {
      const formData = this.getFormData(this.$formFields);

      while (formData.handle.startsWith('@')) {
        formData.handle = formData.handle.slice(1);
      }

      if (formData.colors) {
        Object.keys(formData.colors)
          .forEach((colorField) => {
            if (!formData.colors[colorField].startsWith('#')) {
              formData.colors[colorField] = `#${formData.colors[colorField]}`;
            }
          });
      }

      return formData;
    },

    save () {
      const formData = this.getFormDataEx();

      // set the model data for the social accounts
      this.socialAccounts.setCollectionData();

      this.profile.set(formData);

      const save = this.profile.save();
      let saveAvatar;
      let saveHeader;

      if (save) {
        if (this.avatarOffsetOnLoad !== this.avatarCropper.cropit('offset')
          || this.avatarZoomOnLoad !== this.avatarCropper.cropit('zoom')) {
          this.avatarChanged = true;
        }

        if (this.headerOffsetOnLoad !== this.headerCropper.cropit('offset')
          || this.headerZoomOnLoad !== this.headerCropper.cropit('zoom')) {
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
            this.$btnSave.removeClass('processing');
            setTimeout(() => statusMessage.remove(), 3000);
          });
      }

      this.render();

      if (save) {
        this.$btnSave.addClass('processing');
      } else {
        const $firstErr = $('.errorList:first');

        if ($firstErr.length) {
          $firstErr[0].scrollIntoViewIfNeeded();
        } else {
          this.$emit('unrecognizedModelError', this, [this.profile]);
        }
      }
    },

    get $btnSave () {
      if (!this._$btnSave) {
        this._$btnSave = $('.js-save');
      }
      return this._$btnSave;
    },

    render () {
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

      const formFields = 'select[name], input[name], textarea[name], div[contenteditable][name]';
      this.$formFields = $(formFields);
      this._$btnSave = null;

      installRichEditor($('#settingsAbout'), {
        topLevelClass: 'clrBr',
      });

      const avatarPrev = $('.js-avatarPreview');
      const avatarInpt = $('#avatarInput');
      this.avatarCropper = $('#avatarCropper');

      const headerPrev = $('.js-headerPreview');
      const headerInpt = $('#headerInput');
      this.headerCropper = $('#headerCropper');

      // if the avatar or header exist, don't count the first load as a change
      this.avatarLoadedOnRender = Boolean(avatarURI || this.profile.get('avatarHashes').get('original'));
      this.headerLoadedOnRender = Boolean(headerURI || this.profile.get('headerHashes').get('original'));

      setTimeout(() => {
        this.avatarCropper.cropit({
          $preview: avatarPrev,
          $fileInput: avatarInpt,
          smallImage: 'stretch',
          allowDragNDrop: false,
          maxZoom: 2,
          onImageLoaded: () => {
            this.avatarOffsetOnLoad = this.avatarCropper.cropit('offset');
            this.avatarZoomOnLoad = this.avatarCropper.cropit('zoom');
            $('.js-avatarLeft').removeClass('disabled');
            $('.js-avatarRight').removeClass('disabled');
            $('.js-avatarZoom').removeClass('disabled');
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
          $preview: headerPrev,
          $fileInput: headerInpt,
          smallImage: 'stretch',
          allowDragNDrop: false,
          maxZoom: 2,
          onImageLoaded: () => {
            this.headerOffsetOnLoad = this.headerCropper.cropit('offset');
            this.headerZoomOnLoad = this.headerCropper.cropit('zoom');
            $('.js-headerLeft').removeClass('disabled');
            $('.js-headerRight').removeClass('disabled');
            $('.js-headerZoom').removeClass('disabled');
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
          this.avatarCropper.cropit(
            'imageSrc',
            app.getServerUrl(`ob/image/${this.profile.get('avatarHashes').get('original')}`),
          );
        }

        if (headerURI) {
          this.headerCropper.cropit('imageSrc', headerURI);
        } else if (this.profile.get('headerHashes').get('original')) {
          this.headerCropper.cropit(
            'imageSrc',
            app.getServerUrl(`ob/image/${this.profile.get('headerHashes').get('original')}`),
          );
        }
      }, 0);

      this.socialAccounts.delegateEvents();
      $('.js-socialAccounts').append(this.socialAccounts.render().el);

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
