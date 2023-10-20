<template>
  <div
    :class="`modal editListing tabbedModal modalScrollPage ${contractTypeClass} ${!createMode ? 'editMode' : ''} ${fixedNav ? 'fixedNav' : ''} ${
      notTrackingInventory ? 'notTrackingInventory' : ''
    }`"
    @scroll="onScroll"
  >
    <BaseModal @close="close">
      <template v-slot:component>
        <div class="topControls flex">
          <div class="btnStrip clrSh3">
            <template v-if="ob.returnText">
              <a class="btn clrP clrBr clrT" @click="onClickReturn">
                <span class="ion-chevron-left margRSm"></span>
                {{ ob.returnText }}
              </a>
            </template>
          </div>
        </div>

        <div class="flex gutterH">
          <div class="tabColumn contentBox padMd clrP clrBr clrSh3">
            <div class="boxList tx4 clrTx1Br">
              <a v-for="(tab, index) in tabs" :key="index" @click="scrollTo(tab)" :class="`tab row tab-${tab.key} ${activeTab === tab.key ? 'active' : ''}`">{{
                tab.name
              }}</a>
            </div>
          </div>
          <div class="flexExpand posR tabContent">
            <div class="gutterVMd2 js-formSectionsContainer">
              <section ref="sectionGeneral" class="generalSection contentBox padMd clrP clrBr clrSh3">
                <div class="flexHCent">
                  <h2 class="h3 clrT js-listingHeading">
                    {{ ob.createMode ? ob.polyT('editListing.createListingLabel') : ob.polyT('editListing.editListingLabel') }}
                  </h2>
                  <a class="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn" @click="onSaveClick">{{ ob.polyT('settings.btnSave') }}</a>
                </div>
                <hr class="clrBr" />

                <div class="tabFormWrapper">
                  <form class="box padSmKids padStack">
                    <div class="standardTypeWrap js-standardTypeWrap pad0 padSmKids padStackAll">
                      <div class="flexRow">
                        <div class="col12">
                          <div class="flexRow">
                            <label for="editListingTitle" class="required flexExpand">{{ ob.polyT('editListing.title') }}</label>
                            <ViewListingLinks :createMode="ob.createMode" />
                          </div>
                          <FormError v-if="ob.errors['item.title']" :errors="ob.errors['item.title']" />
                          <input
                            type="text"
                            class="clrBr clrP clrSh2"
                            name="item.title"
                            id="editListingTitle"
                            :value="ob.item.title"
                            :maxLength="ob.max.title"
                            :placeholder="ob.polyT('editListing.placeholderTitle')"
                          />
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperTitle') }}</div>
                        </div>
                      </div>
                      <div class="flexRow gutterH">
                        <div class="col6 simpleFlexCol">
                          <label for="editContractType" class="required">{{ ob.polyT('editListing.type') }}</label>
                          <FormError v-if="ob.errors['metadata.contractType']" :errors="ob.errors['metadata.contractType']" />
                          <select
                            id="editContractType"
                            @change="onChangeContractType(val)"
                            name="metadata.contractType"
                            class="clrBr clrP clrSh2 marginTopAuto"
                            style="width: 100%"
                          >
                            <template v-for="(contractType, j) in ob.contractTypes" :key="j">
                              <option :value="contractType.code" :selected="contractType.code === ob.metadata.contractType">{{ contractType.name }}</option>
                            </template>
                          </select>
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperType', { smart_count: 4 }) }}</div>
                        </div>
                        <div class="col6 simpleFlexCol">
                          <!-- // hiding until this is ready on the back end -->
                          <div class="hide">
                            <label for="editListingVisibility" class="required">{{ ob.polyT('editListing.visibility') }}</label>
                            <select id="editListingVisibility" class="clrBr clrP clrSh2 marginTopAuto">
                              <option value="hidden">Hidden (doesn't display in store)</option>
                            </select>
                          </div>
                        </div>
                      </div>
                      <div class="flexRow gutterH">
                        <div class="col6 simpleFlexCol">
                          <label for="editListingPrice" class="required">{{ ob.polyT('editListing.price') }}</label>
                          <FormError v-if="ob.errors['item.price']" :errors="ob.errors['item.price']" />
                          <FormError v-if="ob.errors['metadata.pricingCurrency.code']" :errors="ob.errors['metadata.pricingCurrency.code']" />
                          <div class="inputSelect marginTopAuto">
                            <input
                              type="text"
                              class="clrBr clrP clrSh2"
                              @change="onChangePrice"
                              name="item.price"
                              id="editListingPrice"
                              :value="ob.number.toStandardNotation(ob.item.price)"
                              placeholder="0.00"
                              data-var-type="bignumber"
                            />
                            <select id="editListingCurrency" name="metadata.pricingCurrency.code" class="clrBr clrP nestInputRight">
                              <template v-for="(currency, j) in ob.currencies" :key="j">
                                <option :value="currency.code" :data-name="currency.name" :selected="currency.code === ob.listingCurrency">
                                  {{ currency.code }}
                                </option>
                              </template>
                            </select>
                          </div>
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperPrice', { cur: helperCryptoCurName }) }}</div>
                        </div>
                        <div class="col6 simpleFlexCol conditionWrap">
                          <label for="editListingCondition" class="required">{{ ob.polyT('editListing.condition') }}</label>
                          <FormError v-if="ob.errors['item.condition']" :errors="ob.errors['item.condition']" />
                          <select id="editListingCondition" name="item.condition" class="clrBr clrP clrSh2 marginTopAuto" style="width: 100%">
                            <template v-for="(conditionType, j) in ob.conditionTypes" :key="j">
                              <option :value="conditionType.code" :selected="conditionType.code === ob.item.condition">{{ conditionType.name }}</option>
                            </template>
                          </select>
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperCondition') }}</div>
                        </div>
                      </div>
                    </div>
                    <div class="cryptoTypeWrap js-cryptoTypeWrap pad0"></div>
                    <div class="flexRow gutterH skuMatureContentRow js-skuMatureContentRow">
                      <div class="col6 simpleFlexCol js-skuFieldContainer"></div>
                      <div class="col6 simpleFlexCol">
                        <label>{{ ob.polyT('editListing.nsfwLabel') }}</label>
                        <FormError v-if="ob.errors['item.nsfw']" :errors="ob.errors['item.nsfw']" />
                        <div class="btnStrip">
                          <div class="btnRadio clrBr">
                            <input type="radio" name="item.nsfw" value="true" id="editListingNSFWInputTrue" data-var-type="boolean" :checked="ob.item.nsfw" />
                            <label for="editListingNSFWInputTrue">{{ ob.polyT('editListing.nsfwYes') }}</label>
                          </div>
                          <div class="btnRadio clrBr">
                            <input
                              type="radio"
                              name="item.nsfw"
                              value="false"
                              id="editListingNSFWInputFalse"
                              data-var-type="boolean"
                              :checked="!ob.item.nsfw"
                            />
                            <label for="editListingNSFWInputFalse">{{ ob.polyT('editListing.nsfwNo') }}</label>
                          </div>
                        </div>
                        <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperNSFW') }}</div>
                      </div>
                    </div>
                    <div class="flexRow gutterH">
                      <div class="col12">
                        <label for="editListingDescription">{{ ob.polyT('editListing.description') }}</label>
                        <FormError v-if="ob.errors['item.description']" :errors="ob.errors['item.description']" />
                        <div
                          contenteditable
                          class="clrBr clrSh2"
                          name="item.description"
                          id="editListingDescription"
                          :placeholder="ob.polyT('editListing.placeholderDescription')"
                        >
                          {{ ob.item.description }}
                        </div>
                      </div>
                    </div>
                  </form>
                </div>
              </section>

              <section ref="sectionPhotos" class="photosSection photoUploadSection contentBox padMd clrP clrBr clrSh3 tx3">
                <div class="overflowAuto">
                  <h2 class="h4 clrT required">{{ ob.polyT('editListing.sectionNames.photos') }}</h2>
                  <div class="js-photoUploadingLabel floR" v-show="!!ob.photoUploadInprogress">
                    {{ ob.polyT('editListing.uploading') }} <a class="" @click="onClickCancelPhotoUploads">{{ ob.polyT('editListing.btnCancelUpload') }}</a>
                  </div>
                  <hr class="clrBr rowMd" />
                </div>
                <FormError v-if="ob.errors['item.images']" :errors="ob.errors['item.images']" />
                <input type="file" id="inputPhotoUpload" @change="onChangePhotoUploadInput" accept="image/*" class="hide" multiple />
                <ul ref="photoUploadItems" class="unstyled uploadItems clrBr rowSm js-photoUploadItems">
                  <li class="addElement tile js-addPhotoWrap">
                    <span class="imagesIcon ion-images clrT4"></span>
                    <button class="btn clrP clrBr clrT tx6" @click="onClickAddPhoto">{{ ob.polyT('editListing.btnAddPhoto') }}</button>
                  </li>
                  <template v-for="(image, j) in ob.item.images">
                    <UploadPhoto :image="image" @closeIcon="onClickRemoveImage(j)" />
                  </template>
                </ul>
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperPhotos', { maxPhotos: ob.max.photos }) }}</div>
              </section>

              <section ref="sectionShipping" class="shippingSection js-sectionShipping">
                <div class="gutterVMd">
                  <div class="js-shippingOptionsWrap shippingOptionsWrap gutterVMd"></div>
                  <div class="contentBox padMd clrP clrBr clrSh3 tx3 shipOptPlaceholder">
                    <FormError v-if="ob.errors['shippingOptions']" :errors="ob.errors['shippingOptions']" :class="topLevelShipOptErrs" />
                    <h2 class="h4 clrT js-addShipOptSectionHeading">
                      {{ ob.polyT('editListing.shippingOptions.optionHeading', { listPosition: ob.shippingOptions.length + 1 }) }}
                    </h2>
                    <hr class="clrBr rowMd" />
                    <a class="btn clrBr clrP clrSh2 rowSm" @click="onClickAddShippingOption">{{
                      ob.polyT('editListing.shippingOptions.btnAddShippingOption')
                    }}</a>
                    <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperShipping') }}</div>
                  </div>
                </div>
              </section>

              <section ref="sectionTags" class="tagsSection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.tagsDetailed') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['item.tags']" :errors="ob.errors['item.tags']" />
                <div class="js-maxTagsWarning"><div v-if="ob.item.tags.length >= ob.max.tags" v-html="ob.maxTagsWarning" /></div>
                <input
                  type="text"
                  id="editListingTags"
                  name="item.tags"
                  class="clrBr clrP hashPrefacedTags hideDropDown"
                  :value="ob.item.tags.join(ob.tagsDelimiter)"
                  :placeholder="ob.polyT('editListing.tagsPlaceholder')"
                />
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperTags') }}</div>
              </section>

              <section ref="sectionCategory" class="categorySection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.categoryDetailed') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['item.categories']" :errors="ob.errors['item.categories']" />
                <div class="js-maxCatsWarning"><div v-if="ob.item.categories.length >= ob.max.cats" v-html="ob.maxCatsWarning" /></div>
                <input
                  type="text"
                  id="editListingCategories"
                  name="item.categories"
                  class="clrBr clrP hideDropDown"
                  :value="ob.item.categories.join(ob.tagsDelimiter)"
                  :placeholder="ob.polyT('editListing.categoryPlaceholder')"
                />
              </section>

              <section
                ref="sectionVariants"
                class="variantsSection js-variantsSection contentBox padMd clrP clrBr clrSh3 tx3 <% ob.item.options.length && print('expandedVariantsView') %>"
              >
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.variantsDetailed') }}</h2>
                <hr class="clrBr rowMd" />
                <div class="js-variantsContainer variantsContainer"></div>
                <a class="btn clrP clrBr clrSh2 addFirstVariant" @click="onClickAddFirstVariant">{{ ob.polyT('editListing.variants.btnAddVariant') }}</a>
              </section>

              <section class="contentBox padMd clrP clrBr clrSh3 tx3 js-inventoryManagementSection inventoryManagementSection"></section>

              <section
                ref="sectionVariantInventory"
                class="contentBox variantInventorySection js-variantInventorySection padMd clrP clrBr clrSh3 tx3"
                v-show="!!ob.shouldShowVariantInventorySection"
              >
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.variantInventory') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['item.skus']" :errors="ob.errors['item.skus']" />
                <div class="js-variantInventoryTableContainer"></div>
              </section>

              <section ref="sectionReturnPolicy" class="returnPolicySection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.returnPolicy') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['refundPolicy']" :errors="ob.errors['refundPolicy']" />
                <a class="btn clrP clrBr clrSh2 rowSm %>" v-show="!ob.expandedReturnPolicy" @click="onClickAddReturnPolicy">{{
                  ob.polyT('editListing.btnAddReturnPolicy')
                }}</a>
                <textarea
                  rows="8"
                  name="refundPolicy"
                  class="clrBr clrP clrSh2 <% !ob.expandedReturnPolicy && print('hide') %>"
                  id="editListingReturnPolicy"
                  :placeholder="ob.polyT('editListing.placeholderReturnPolicy')"
                  >{{ ob.refundPolicy }}</textarea
                >
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperReturnPolicy') }}</div>
              </section>

              <section ref="sectionTermsAndConditions" class="termsAndConditionsSection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.termsAndConditions') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['termsAndConditions']" :errors="ob.errors['termsAndConditions']" />
                <a class="btn clrP clrBr clrSh2 rowSm <% ob.expandedTermsAndConditions && print('hide') %>" @click="onClickAddTermsAndConditions">{{
                  ob.polyT('editListing.btnTermsAndConditions')
                }}</a>
                <textarea
                  rows="8"
                  name="termsAndConditions"
                  class="clrBr clrP clrSh2 <% !ob.expandedTermsAndConditions && print('hide') %>"
                  id="editListingTermsAndConditions"
                  :placeholder="ob.polyT('editListing.placeholderTerms')"
                  >{{ ob.termsAndConditions }}</textarea
                >
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperTerms') }}</div>
              </section>

              <section
                ref="sectionCoupons"
                class="couponsSection contentBox padMd clrP clrBr clrSh3 tx3 js-couponsSection <% ob.coupons.length && print('expandedCouponView') %>"
              >
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.coupons') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['coupons']" :errors="ob.errors['coupons']" />
                <div class="js-couponsContainer couponsContainer"></div>
                <a class="btn clrP clrBr clrSh2 btnAddCoupon" @click="onClickAddCoupon">{{ ob.polyT('editListing.btnAddCoupon') }}</a>
              </section>

              <section ref="sectionAcceptedCurs" class="acceptedCursSection contentBox padMd clrP clrBr clrSh3 tx3 acceptedCurrenciesSection">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.acceptedCurrencies') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError
                  v-if="ob.errors['metadata.acceptedCurrencies'] && ob.metadata.contractType !== 'CRYPTOCURRENCY'"
                  :errors="ob.errors['metadata.acceptedCurrencies']"
                />
                <div class="js-cryptoCurSelectContainer rowSm">
                  <CryptoCurSelector
                    ref="cryptoCurSelector"
                    :options="{
                      initialState: {
                        currencies: [...ob.metadata.acceptedCurrencies, ...supportedWalletCurs()],
                        activeCurs: ob.metadata.acceptedCurrencies,
                        sort: true,
                      },
                    }"
                  />
                </div>
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperAcceptedCurrencies') }}</div>
              </section>

              <div class="contentBox padMd clrP clrBr clrSh3">
                <div class="flexHRight flexVCent gutterH">
                  <ViewListingLinks :createMode="ob.createMode" />
                  <a class="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph" @click="onSaveClick">{{ ob.polyT('settings.btnSave') }}</a>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import Velocity from 'velocity-animate';

import Sortable from 'sortablejs';
import _ from 'underscore';
import path from 'path';
import Backbone from 'backbone';
import { tagsDelimiter } from '../../../../backbone/utils/lib/selectize';
import 'velocity-animate/velocity.ui';
import app from '../../../../backbone/app';
import { isScrolledIntoView, openExternal } from '../../../../backbone/utils/dom';
import { installRichEditor } from '../../../../backbone/utils/lib/trumbowyg';
import { startAjaxEvent, endAjaxEvent } from '../../../../backbone/utils/metrics';
import { getCurrenciesSortedByCode, getCurrencyByCode } from '../../../../backbone/data/currencies';
import {
  getCurrenciesSortedByName as getCryptoCursByName,
  getCurrenciesSortedByCode as getCryptoCursByCode,
} from '../../../../backbone/data/cryptoListingCurrencies';
import { supportedWalletCurs } from '../../../../backbone/data/walletCurrencies';
import { getCoinDivisibility } from '../../../../backbone/utils/currency';
import { setDeepValue } from '../../../../backbone/utils/object';
import SimpleMessage, { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import Dialog from '../../../../backbone/views/modals/Dialog';
import ShippingOptionMd from '../../../../backbone/models/listing/ShippingOption';
import Service from '../../../../backbone/models/listing/Service';
import Image from '../../../../backbone/models/listing/Image';
import Coupon from '../../../../backbone/models/listing/Coupon';
import VariantOption from '../../../../backbone/models/listing/VariantOption';
import ShippingOption from '../../../../backbone/views/modals/editListing/ShippingOption';
import Coupons from '../../../../backbone/views/modals/editListing/Coupons';
import Variants from '../../../../backbone/views/modals/editListing/Variants';
import VariantInventory from '../../../../backbone/views/modals/editListing/VariantInventory';
import InventoryManagement from '../../../../backbone/views/modals/editListing/InventoryManagement';
import SkuField from '../../../../backbone/views/modals/editListing/SkuField';
import UnsupportedCurrency from '../../../../backbone/views/modals/editListing/UnsupportedCurrency';
import CryptoCurrencyType from '../../../../backbone/views/modals/editListing/CryptoCurrencyType';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
import { capitalize } from '../../../../backbone/utils/string';

import ViewListingLinks from './ViewListingLinks.vue';
import UploadPhoto from './UploadPhoto.vue';

export default {
  components: {
    ViewListingLinks,
    UploadPhoto,
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
      activeTab: 'general',

      contractTypeClass: '',
      images: undefined,

      fixedNav: false,
      notTrackingInventory: true,

      togglePhotoUploads: false,

      currencies: [],
      expandedReturnPolicy: false,
      expandedTermsAndConditions: false,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.render();
  },
  unmounted() {
    this.inProgressPhotoUploads.forEach((upload) => upload.abort());
    $(window).off('resize', this.throttledResizeWin);
  },
  computed: {
    ob() {
      const item = this.model.get('item');
      const metadata = this.model.get('metadata');

      this.currencies = this.currencies || getCurrenciesSortedByCode();

      return {
        ...this.templateHelpers,
        createMode: this.createMode,
        returnText: this.options.returnText,
        listingCurrency: this.currency,
        countryList: this.countryList,
        currencies: this.currencies,
        contractTypes: metadata.contractTypesVerbose,
        conditionTypes: this._model.item.conditionTypes ? this._model.item.conditionTypes.map((conditionType) => ({
          code: conditionType,
          name: app.polyglot.t(`conditionTypes.${conditionType}`),
        })) : [],
        errors: this.model.validationError || {},
        photoUploadInprogress: !!this.inProgressPhotoUploads.length,
        expandedReturnPolicy: this.expandedReturnPolicy || !!this._model.refundPolicy,
        expandedTermsAndConditions: this.expandedTermsAndConditions || !!this._model.termsAndConditions,
        maxCatsWarning: this.maxCatsWarning,
        maxTagsWarning: this.maxTagsWarning,
        max: {
          title: item.max.titleLength,
          cats: item.max.cats,
          tags: item.max.tags,
          photos: this.MAX_PHOTOS,
        },
        shouldShowVariantInventorySection: this.shouldShowVariantInventorySection,
        ...this._model,
      };
    },
    tabs() {
      const ob = this.ob;
      return [
        {
          key: 'general',
          name: ob.polyT('editListing.sectionNames.general'),
        },
        {
          key: 'photos',
          name: ob.polyT('editListing.sectionNames.photos'),
        },
        {
          key: 'shipping',
          name: ob.polyT('editListing.sectionNames.shipping'),
        },
        {
          key: 'tags',
          name: ob.polyT('editListing.sectionNames.tags'),
        },
        // {
        //   key: 'shippingOrigin',
        //   name: ob.polyT('editListing.sectionNames.sendLocation'),
        // },
        {
          key: 'category',
          name: ob.polyT('editListing.sectionNames.category'),
        },
        {
          key: 'variants',
          name: ob.polyT('editListing.sectionNames.variants'),
        },
        {
          key: 'returnPolicy',
          name: ob.polyT('editListing.sectionNames.returnPolicy'),
        },
        {
          key: 'termsAndConditions',
          name: ob.polyT('editListing.sectionNames.termsAndConditions'),
        },
        {
          key: 'coupons',
          name: ob.polyT('editListing.sectionNames.coupons'),
        },
        {
          key: 'acceptedCurs',
          name: ob.polyT('editListing.sectionNames.acceptedCurrencies'),
        },
      ];
    },

    helperCryptoCurName() {
      const ob = this.ob;

      const supportedWalletCurs = ob.crypto.supportedWalletCurs().map((cur) => ob.crypto.ensureMainnetCode(cur));
      const helperCryptoCurCode = supportedWalletCurs.includes('BTC') ? 'BTC' : supportedWalletCurs.sort()[0] || 'EUR';
      return ob.polyT(`cryptoCurrencies.${helperCryptoCurCode}`, ob.polyT(`currencies.${helperCryptoCurCode}`, { _: helperCryptoCurCode }));
    },

    MAX_PHOTOS() {
      return this.model.get('item').max.images;
    },

    shouldShowVariantInventorySection() {
      return !!this.variantOptionsCl.length;
    },

    inProgressPhotoUploads() {
      let access = this.togglePhotoUploads;

      return this.photoUploads.filter((upload) => upload.state() === 'pending');
    },

    trackInventoryBy() {
      let trackBy;

      // If the inventoryManagement has been rendered, we'll let it's drop-down
      // determine whether we are tracking inventory. Otherwise, we'll get the info
      // from the model.
      if (this.inventoryManagement) {
        trackBy = this.inventoryManagement.getState().trackBy;
      } else {
        const item = this.model.get('item');

        if (item.isInventoryTracked) {
          trackBy = item.get('options').length ? 'TRACK_BY_VARIANT' : 'TRACK_BY_FIXED';
        } else {
          trackBy = 'DO_NOT_TRACK';
        }
      }

      return trackBy;
    },

    // return the currency associated with this listing
    currency() {
      if (this.$currencySelect.length) {
        return this.$currencySelect.val();
      }

      let cur = app.settings.get('localCurrency');

      try {
        cur = this.model.get('metadata').get('pricingCurrency').code;
      } catch (e) {
        // pass
      }

      return cur;
    },

    // Keep in mind this could return undefined if certain dependant form fields are not set yet
    // (e.g. rendering not complete, dependant async data not loaded) and the divisibility was
    // never set in the model.
    coinDivisibility() {
      let coinDiv;

      if ($('#editContractType').length) {
        try {
          coinDiv = getCoinDivisibility(
            $('#editContractType').val() === 'CRYPTOCURRENCY' ? $('#editListingCoinType').val() || this.model.get('metadata').get('coinType') : this.currency
          );
        } catch (e) {
          // pass
        }
      } else {
        coinDiv = this.model.get('metadata').get('coinDivisibility');
      }

      return coinDiv;
    },

    maxCatsWarning() {
      return `<div class="clrT2 tx5 row">${app.polyglot.t('editListing.maxCatsWarning')}</div>`;
    },

    $formFields() {
      const isCrypto = $('#editContractType').val() === 'CRYPTOCURRENCY';
      const cryptoExcludes = isCrypto ? ', .js-inventoryManagementSection' : '';
      const excludes = '.js-sectionShipping, .js-couponsSection, .js-variantsSection, ' + `.js-variantInventorySection${cryptoExcludes}`;

      let $fields = $(
        `.js-formSectionsContainer > section:not(${excludes}) select[name],` +
          `.js-formSectionsContainer > section:not(${excludes}) input[name],` +
          `.js-formSectionsContainer > section:not(${excludes}) div[contenteditable][name],` +
          `.js-formSectionsContainer > section:not(${excludes}) ` +
          'textarea[name]:not([class*="trumbowyg"])'
      );

      // Filter out hidden fields that are not applicable based on whether this is
      // a crypto currency listing.
      $fields = $fields.filter((index, el) => {
        const $excludeContainers = isCrypto ? $('.js-standardTypeWrap').add($('.js-skuMatureContentRow')) : $('.js-cryptoTypeWrap');

        let keep = true;

        $excludeContainers.each((i, container) => {
          if ($.contains(container, el)) {
            keep = false;
          }
        });

        return keep;
      });

      return $fields;
    },

    $currencySelect() {
      return this._$currencySelect || (this._$currencySelect = $('#editListingCurrency'));
    },

    $priceInput() {
      return this._$priceInput || (this._$priceInput = $('#editListingPrice'));
    },

    $saveButton() {
      return this._$buttonSave || (this._$buttonSave = $('.js-save'));
    },

    $inputPhotoUpload() {
      return this._$inputPhotoUpload || (this._$inputPhotoUpload = $('#inputPhotoUpload'));
    },

    $photoUploadingLabel() {
      return this._$photoUploadingLabel || (this._$photoUploadingLabel = $('.js-photoUploadingLabel'));
    },

    $editListingReturnPolicy() {
      return this._$editListingReturnPolicy || (this._$editListingReturnPolicy = $('#editListingReturnPolicy'));
    },

    $editListingTermsAndConditions() {
      return this._$editListingTermsAndConditions || (this._$editListingTermsAndConditions = $('#editListingTermsAndConditions'));
    },

    $sectionShipping() {
      return this._$sectionShipping || (this._$sectionShipping = $('.js-sectionShipping'));
    },

    $maxCatsWarning() {
      return this._$maxCatsWarning || (this._$maxCatsWarning = $('.js-maxCatsWarning'));
    },

    $maxTagsWarning() {
      return this._$maxTagsWarning || (this._$maxTagsWarning = $('.js-maxTagsWarning'));
    },

    maxTagsWarning() {
      return `<div class="clrT2 tx5 row">${app.polyglot.t('editListing.maxTagsWarning')}</div>`;
    },

    $addShipOptSectionHeading() {
      return this._$addShipOptSectionHeading || (this._$addShipOptSectionHeading = $('.js-addShipOptSectionHeading'));
    },

    $variantInventorySection() {
      return this._$variantInventorySection || (this._$variantInventorySection = $('.js-variantInventorySection'));
    },

    $itemPrice() {
      return this._$itemPrice || (this._$itemPrice = $('[name="item.price"]'));
    },
  },
  methods: {
    supportedWalletCurs,
    loadData(options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      if (options.onClickViewListing !== undefined && typeof options.onClickViewListing !== 'function') {
        throw new Error('If providing an onClickViewListing option, it must be ' + 'provided as a function.');
      }

      this.baseInit(options);

      // So the passed in model does not get any un-saved data,
      // we'll clone and update it on sync
      this._origModel = this.model;
      this.model = this._origModel.clone();

      this.listenTo(this.model, 'sync', () => {
        setTimeout(() => {
          if (this.createMode && !this.model.isNew()) {
            this.createMode = false;
            $('.js-listingHeading').text(app.polyglot.t('editListing.editListingLabel'));
          }

          const updatedData = this.model.toJSON();

          // Will parse out some sku attributes that are specific to the variant
          // inventory view.
          updatedData.item.skus = updatedData.item.skus.map((sku) => _.omit(sku, 'mappingId', 'choices'));

          if (updatedData.item.quantity === undefined) {
            this._origModel.get('item').unset('quantity');
          }

          if (updatedData.item.productID === undefined) {
            this._origModel.get('item').unset('productID');
          }

          this._origModel.set(updatedData);
        });

        // A change event won't fire on a parent model if only nested attributes change.
        // The nested models would need to have change events manually bound to them
        // which is cumbersome with a model like this with so many levels of nesting.
        // If you are interested in any change on the model (as opposed to a specific
        // attribute), the simplest thing to do is use the 'saved' event from the
        // event emitter in models/listing/index.js.
      });

      this.createMode = !(this.model.lastSyncedAttrs && this.model.lastSyncedAttrs.slug);
      this.photoUploads = [];
      this.images = this.model.get('item').get('images');
      this.shippingOptions = this.model.get('shippingOptions');
      this.shippingOptionViews = [];
      this.getCoinTypesDeferred = $.Deferred();
      this.countryList = getTranslatedCountries();

      // Since the UI is driven from the model and since the Receive field
      // and the Accepted Currencies select list are both driven by the same
      // model field (acceptdCurrencies), we'll keep track of their values
      // seperately, so they don't interfere with each other.
      const getAcceptedCurs = () => this.model.get('metadata').get('acceptedCurrencies');
      const getReceiveCur = () => (this.model.isCrypto ? (getAcceptedCurs().length && getAcceptedCurs()[0]) || null : null);
      this._receiveCryptoCur = getReceiveCur();
      this._acceptedCurs = getAcceptedCurs();
      this.listenTo(
        this.model.get('metadata'),
        'change:acceptedCurrencies',

        () => {
          if (this.model.isCrypto) {
            this._receiveCryptoCur = getReceiveCur();
          } else {
            this._acceptedCurs = getAcceptedCurs();
          }
        }
      );

      getCryptoCursByName().then(
        (curs) => this.getCoinTypesDeferred.resolve(curs),
        () => this.getCoinTypesDeferred.resolve(getCryptoCursByCode().map((cur) => ({ code: cur, name: cur })))
      );

      this.listenTo(this.shippingOptions, 'add', (shipOptMd) => {
        const shipOptVw = this.createShippingOptionView({
          listPosition: this.shippingOptions.length,
          model: shipOptMd,
        });

        this.shippingOptionViews.push(shipOptVw);
        this.$shippingOptionsWrap.append(shipOptVw.render().el);
      });

      this.listenTo(this.shippingOptions, 'remove', (shipOptMd, shipOptCl, removeOpts) => {
        const [splicedVw] = this.shippingOptionViews.splice(removeOpts.index, 1);
        splicedVw.remove();
        this.shippingOptionViews.slice(removeOpts.index).forEach((shipOptVw) => {
          shipOptVw.listPosition -= 1;
        });
      });

      this.listenTo(this.shippingOptions, 'update', (cl, updateOpts) => {
        if (!(updateOpts.changes.added.length || updateOpts.changes.removed.length)) {
          return;
        }

        this.$addShipOptSectionHeading.text(app.polyglot.t('editListing.shippingOptions.optionHeading', { listPosition: this.shippingOptions.length + 1 }));
      });

      this.coupons = this.model.get('coupons');
      this.listenTo(this.coupons, 'update', () => {
        if (this.coupons.length) {
          this.$couponsSection.addClass('expandedCouponView');
        } else {
          this.$couponsSection.removeClass('expandedCouponView');
        }
      });

      this.variantOptionsCl = this.model.get('item').get('options');

      this.listenTo(this.variantOptionsCl, 'update', this.onUpdateVariantOptions);

      if (this.trackInventoryBy === 'DO_NOT_TRACK') {
        this.notTrackingInventory = true;
      }
    },

    events() {
      return {
        'change #editListingCryptoContractType': 'onChangeCryptoContractType',
        'click .js-removeImage': 'onClickRemoveImage',
        'keyup .js-variantNameInput': 'onKeyUpVariantName',
        'click .js-scrollToVariantInventory': 'onClickScrollToVariantInventory',
        'click .js-viewListing': 'onClickViewListing',
        'click .js-viewListingOnWeb': 'onClickViewListingOnWeb',
      };
    },

    onClickReturn() {
      this.trigger('click-return', { view: this });
    },

    onClickViewListing() {
      if (this.options.onClickViewListing) {
        this.options.onClickViewListing.call(this);
      } else {
        const slug = this.model.get('slug');
        if (slug) {
          app.router.navigate(`${app.profile.id}/store/${slug}`, { trigger: true });
        } else {
          throw new Error('There is no slug for this listing in order to navigate!');
        }
      }
    },

    onClickViewListingOnWeb() {
      const slug = this.model.get('slug');
      if (slug) {
        openExternal(`https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info/listing/${app.profile.id}/${slug}`);
      } else {
        throw new Error('There is no slug for this listing in order to navigate!');
      }
    },

    onClickRemoveImage(j) {
      this.images.remove(this.images.at(j));
    },

    onClickCancelPhotoUploads() {
      this.inProgressPhotoUploads.forEach((photoUpload) => photoUpload.abort());
    },

    onChangePrice() {
      this.variantInventory.render();
    },

    setContractTypeClass(contractType) {
      this.contractTypeClass = `TYPE_${contractType}`;
    },

    onChangeContractType(val, data = {}) {
      this.setContractTypeClass(val);

      if (!data.fromCryptoTypeChange) {
        if (val === 'CRYPTOCURRENCY') {
          this.model.get('metadata').set('acceptedCurrencies', [this._receiveCryptoCur]);
          $('#editListingCryptoContractType').val('CRYPTOCURRENCY');
          $('#editListingCryptoContractType').trigger('change').focus();
        }
      }
    },

    onChangeCryptoContractType(e) {
      if (e.target.value === 'CRYPTOCURRENCY') return;

      this.model.get('metadata').set('acceptedCurrencies', this._acceptedCurs);
      $('#editContractType').val(e.target.value);
      $('#editContractType').trigger('change', { fromCryptoTypeChange: true }).focus();
    },

    getOrientation(file, callback) {
      const reader = new FileReader();

      reader.onload = (e) => {
        const dataView = new DataView(e.target.result); // eslint-disable-line no-undef
        let offset = 2;

        if (dataView.getUint16(0, false) !== 0xffd8) return callback(-2);

        while (offset < dataView.byteLength) {
          const marker = dataView.getUint16(offset, false);
          offset += 2;
          if (marker === 0xffe1) {
            offset += 2;
            if (dataView.getUint32(offset, false) !== 0x45786966) {
              return callback(-1);
            }
            const little = dataView.getUint16((offset += 6), false) === 0x4949;
            offset += dataView.getUint32(offset + 4, little);
            const tags = dataView.getUint16(offset, little);
            offset += 2;
            for (let i = 0; i < tags; i++) {
              if (dataView.getUint16(offset + i * 12, little) === 0x0112) {
                return callback(dataView.getUint16(offset + i * 12 + 8, little));
              }
            }
          } else if ((marker & 0xff00) !== 0xff00) {
            break;
          } else {
            offset += dataView.getUint16(offset, false);
          }
        }

        return callback(-1);
      };

      reader.readAsArrayBuffer(file.slice(0, 64 * 1024));
    },

    truncateImageFilename(filename) {
      if (!filename || typeof filename !== 'string') {
        throw new Error('Please provide a filename as a string.');
      }

      const truncated = filename;

      if (filename.length > Image.maxFilenameLength) {
        const parsed = path.parse(filename);
        const nameParseLen = Image.maxFilenameLength - parsed.ext.length;

        // acounting for rare edge case of the extension in and of itself
        // exceeding the max length
        return parsed.name.slice(0, nameParseLen < 0 ? 0 : nameParseLen) + parsed.ext.slice(0, Image.maxFilenameLength);
      }

      return truncated;
    },

    onChangePhotoUploadInput() {
      let photoFiles = Array.prototype.slice.call(this.$inputPhotoUpload[0].files, 0);

      // prune out any non-image files
      photoFiles = photoFiles.filter((file) => file.type.startsWith('image'));

      this.$inputPhotoUpload.val('');

      const currPhotoLength = this.model.get('item').get('images').length;

      if (currPhotoLength + photoFiles.length > this.MAX_PHOTOS) {
        photoFiles = photoFiles.slice(0, this.MAX_PHOTOS - currPhotoLength);

        new SimpleMessage({
          title: app.polyglot.t('editListing.errors.tooManyPhotosTitle'),
          message: app.polyglot.t('editListing.errors.tooManyPhotosBody'),
        })
          .render()
          .open();
      }

      if (!photoFiles.length) return;

      this.$photoUploadingLabel.removeClass('hide');

      const toUpload = [];
      let loaded = 0;
      let errored = 0;

      photoFiles.forEach((photoFile) => {
        const newImage = document.createElement('img');

        newImage.src = photoFile.path;

        newImage.onload = () => {
          const imgW = newImage.width;
          const imgH = newImage.height;
          const canvas = document.createElement('canvas');
          const ctx = canvas.getContext('2d');

          canvas.width = imgW;
          canvas.height = imgH;

          this.getOrientation(photoFile, (orientation) => {
            if (orientation > 4) {
              canvas.width = imgH;
              canvas.height = imgW;
            }

            switch (orientation) {
              case 2:
                ctx.translate(imgW, 0);
                ctx.scale(-1, 1);
                break;
              case 3:
                ctx.translate(imgW, imgH);
                ctx.rotate(Math.PI);
                break;
              case 4:
                ctx.translate(0, imgH);
                ctx.scale(1, -1);
                break;
              case 5:
                ctx.rotate(0.5 * Math.PI);
                ctx.scale(1, -1);
                break;
              case 6:
                ctx.rotate(0.5 * Math.PI);
                ctx.translate(0, -imgH);
                break;
              case 7:
                ctx.rotate(0.5 * Math.PI);
                ctx.translate(imgW, -imgH);
                ctx.scale(-1, 1);
                break;
              case 8:
                ctx.rotate(-0.5 * Math.PI);
                ctx.translate(-imgW, 0);
                break;
              default: // do nothing
            }

            ctx.drawImage(newImage, 0, 0, imgW, imgH);
            toUpload.push({
              filename: this.truncateImageFilename(photoFile.name),
              image: canvas.toDataURL('image/jpeg', 0.9).replace(/^data:image\/(png|jpeg|webp);base64,/, ''),
            });

            loaded += 1;

            if (loaded + errored === photoFiles.length) {
              this.uploadImages(toUpload);
            }
          });
        };

        newImage.onerror = () => {
          errored += 1;

          if (errored === photoFiles.length) {
            this.$photoUploadingLabel.addClass('hide');

            new SimpleMessage({
              title: app.polyglot.t('editListing.errors.unableToLoadImages', { smart_count: errored }),
            })
              .render()
              .open();
          } else if (loaded + errored === photoFiles.length) {
            this.uploadImages(toUpload);
          }
        };
      });
    },

    onClickAddReturnPolicy(e) {
      $(e.target).addClass('hide');
      this.$editListingReturnPolicy.removeClass('hide').focus();
      this.expandedReturnPolicy = true;
    },

    onClickAddTermsAndConditions(e) {
      $(e.target).addClass('hide');
      this.$editListingTermsAndConditions.removeClass('hide').focus();
      this.expandedTermsAndConditions = true;
    },

    onClickAddShippingOption() {
      this.shippingOptions.push(
        new ShippingOptionMd({
          services: [new Service()],
        })
      );
    },

    onClickAddCoupon() {
      this.coupons.add(new Coupon());

      if (this.coupons.length === 1) {
        this.$couponsSection.find('.coupon input[name=title]').focus();
      }
    },

    onClickAddFirstVariant() {
      this.variantOptionsCl.add(new VariantOption());

      if (this.variantOptionsCl.length === 1) {
        this.$variantsSection.find('.variant input[name=name]').focus();
      }
    },

    onKeyUpVariantName(e) {
      // wait until they stop typing
      if (this.variantNameKeyUpTimer) {
        clearTimeout(this.variantNameKeyUpTimer);
      }

      this.variantNameKeyUpTimer = setTimeout(() => {
        const index = $(e.target).closest('.variant').index();

        this.variantsView.setModelData(index);
      }, 150);
    },

    onVariantChoiceChange(e) {
      const index = this.variantsView.views.indexOf(e.view);

      this.variantsView.setModelData(index);
    },

    onUpdateVariantOptions() {
      if (this.variantOptionsCl.length) {
        this.$variantsSection.addClass('expandedVariantsView');
        this.skuField.setState({ variantsPresent: true });

        if (this.inventoryManagement.getState().trackBy !== 'DO_NOT_TRACK') {
          this.inventoryManagement.setState({
            trackBy: 'TRACK_BY_VARIANT',
          });
        }
      } else {
        this.$variantsSection.removeClass('expandedVariantsView');
        this.skuField.setState({ variantsPresent: false });

        if (this.inventoryManagement.getState().trackBy !== 'DO_NOT_TRACK') {
          this.inventoryManagement.setState({
            trackBy: 'TRACK_BY_FIXED',
          });
        }
      }

      this.$variantInventorySection.toggleClass('hide', !this.shouldShowVariantInventorySection);
    },

    onClickScrollToVariantInventory() {
      this.scrollTo('variantInventory');
    },

    confirmClose() {
      const deferred = $.Deferred();

      this.setModelData();
      const prevData = this.createMode ? this.attrsAtCreate : this.attrsAtLastSave;
      const curData = this.model.toJSON();

      if (!_.isEqual(prevData, curData)) {
        const messageKey = `body${this.createMode ? 'Create' : 'Edit'}`;
        this.closeConfirmDialog = this.createChild(Dialog, {
          removeOnClose: false,
          title: app.polyglot.t('editListing.confirmCloseDialog.title'),
          message: app.polyglot.t(`editListing.confirmCloseDialog.${messageKey}`),
          buttons: [
            {
              text: app.polyglot.t('editListing.confirmCloseDialog.btnNo'),
              fragment: 'no',
            },
            {
              text: app.polyglot.t('editListing.confirmCloseDialog.btnYes'),
              fragment: 'yes',
            },
          ],
        })
          .on('click-yes', () => {
            deferred.resolve();
            this.remove();
          })
          .on('click-no', () => {
            deferred.reject();
            this.closeConfirmDialog.close();
          })
          .on('close', () => deferred.reject())
          .render()
          .open();
      } else {
        deferred.resolve();
      }

      return deferred.promise();
    },

    uploadImages(images) {
      let imagesToUpload = images;

      if (!images) {
        throw new Error('Please provide a list of images to upload.');
      }

      if (typeof images === 'string') {
        imagesToUpload = [images];
      }

      const upload = $.ajax({
        url: app.getServerUrl('ob/images'),
        type: 'POST',
        data: JSON.stringify(imagesToUpload),
        dataType: 'json',
        contentType: 'application/json',
      })
        .always(() => {
          if (this.isRemoved()) return;
          if (!this.inProgressPhotoUploads.length) this.$photoUploadingLabel.addClass('hide');
        })
        .done((uploadedImages) => {
          if (this.isRemoved()) return;

          this.images.add(
            uploadedImages.map((image) => ({
              filename: image.filename,
              original: image.original,
              large: image.large,
              medium: image.medium,
              small: image.small,
              tiny: image.tiny,
            }))
          );
        })
        .fail((jqXhr) => {
          openSimpleMessage(
            app.polyglot.t('editListing.errors.uploadImageErrorTitle', { smart_count: imagesToUpload.length }),
            (jqXhr.responseJSON && jqXhr.responseJSON.reason) || ''
          );
        })
        .always(() => {
          this.togglePhotoUploads = !this.togglePhotoUploads;
        });

      this.photoUploads.push(upload);
    },

    onClickAddPhoto() {
      this.$inputPhotoUpload.trigger('click');
    },

    scrollTo({ key }) {
      this.$scrollTo(`.${key}Section`, 300, {
        container: '.tabbedModal', //设置滚动容器
        easing: 'ease-in', //动画效果
        onDone: () => {
          setTimeout(() => {
            this.activeTab = key;
          }, 20);
        },
        x: false, //是否在x轴滚动
        y: true, //是否在y轴滚动
      });
    },

    _onScroll() {
      for (const tab of this.tabs) {
      if (isScrolledIntoView(this.$refs[`section${capitalize(tab.key)}`])) {
          this.activeTab = tab.key;
          break;
        }
      }

      if (this.$el.scrollTop > 57) {
        this.fixedNav = true;
      } else if (this.$el.scrollTop <= 57) {
        this.fixedNav = false;
      }
    },

    onScroll() {
      _.throttle(this._onScroll, 100)();
    },

    onSaveClick() {
      this.$saveButton.addClass('disabled');
      this.setModelData();

      const serverData = this.model.toJSON();

      serverData.item.skus = serverData.item.skus.map((sku) =>
        // The variant inventory view adds some stuff to the skus collection that
        // shouldn't go to the server. We'll ensure the extraneous stuff isn't sent
        // with the save while still allowing it to stay in the collection.
        _.omit(sku, 'mappingId', 'choices')
      );

      const save = this.model.save(null, {
        attrs: serverData,
      });

      if (save) {
        const segmentation = {
          type: serverData.metadata.contractType,
          currency:
            serverData.metadata.contractType !== 'CRYPTOCURRENCY' ? serverData.metadata.pricingCurrency.code : serverData.item.cryptoListingCurrencyCode,
          moderated: serverData.moderators && !!serverData.moderators.length,
          isNew: this.model.isNew(),
        };

        startAjaxEvent('Listing_Save');

        const savingStatusMsg = app.statusBar
          .pushMessage({
            msg: 'Saving listing...',
            type: 'message',
            duration: 99999999999999,
          })
          .on('clickViewListing', () => {
            const guidUrl = `#${app.profile.id}/store/${this.model.get('slug')}`;
            const base = app.profile.get('handle') ? `@${app.profile.get('handle')}` : app.profile.id;
            const url = `${base}/store/${this.model.get('slug')}`;

            if (location.hash === guidUrl) {
              Backbone.history.loadUrl();
            } else {
              app.router.navigateUser(url, app.profile.id, { trigger: true });
            }
          });

        save
          .always(() => this.$saveButton.removeClass('disabled'))
          .fail((...args) => {
            savingStatusMsg.update({
              msg: `Listing <em>${this.model.toJSON().item.title}</em> failed to save.`,
              type: 'warning',
            });

            setTimeout(() => savingStatusMsg.remove(), 3000);

            const message = (args[0] && args[0].responseJSON && args[0].responseJSON.reason) || '';

            new SimpleMessage({
              title: app.polyglot.t('editListing.errors.saveErrorTitle'),
              message,
            })
              .render()
              .open();
            endAjaxEvent('Listing_Save', {
              ...segmentation,
              errors: message || 'unknown',
            });
          })
          .done(() => {
            savingStatusMsg.update(`Listing ${this.model.toJSON().item.title}` + ' saved. <a class="js-viewListing">view</a>');
            this.attrsAtLastSave = this.model.toJSON();

            setTimeout(() => savingStatusMsg.remove(), 6000);
            endAjaxEvent('Listing_Save', segmentation);
          });
      } else {
        // client side validation failed
        this.$saveButton.removeClass('disabled');
      }

      // render so errrors are shown / cleared
      this.render();

      if (!save) {
        const $firstErr = $('.errorList:visible').eq(0);
        if ($firstErr.length) {
          $firstErr[0].scrollIntoViewIfNeeded();
        } else {
          // There's a model error that's not represented in the UI - likely
          // developer error.
          const msg = Object.keys(this.model.validationError).reduce(
            (str, errKey) => `${str}${errKey}: ${this.model.validationError[errKey].join(', ')}<br>`,
            ''
          );
          openSimpleMessage(app.polyglot.t('editListing.errors.saveErrorTitle'), msg);
        }
      }
    },

    onChangeManagementType(e) {
      if (e.value === 'TRACK') {
        this.inventoryManagement.setState({
          trackBy: this.model.get('item').get('options').length ? 'TRACK_BY_VARIANT' : 'TRACK_BY_FIXED',
        });
        this.notTrackingInventory = false;
      } else {
        this.inventoryManagement.setState({
          trackBy: 'DO_NOT_TRACK',
        });
        this.notTrackingInventory = true;
      }
    },

    /**
     * Will set the model with data from the form, including setting nested models
     * and collections which are managed by nested views.
     */
    setModelData() {
      let formData = this.getFormData(this.$formFields);
      const item = this.model.get('item');
      const metadata = this.model.get('metadata');
      const isCrypto = $('#editContractType').val() === 'CRYPTOCURRENCY';

      // set model / collection data for various child views
      this.shippingOptionViews.forEach((shipOptVw) => shipOptVw.setModelData());
      this.variantsView.setCollectionData();
      this.variantInventory.setCollectionData();
      this.couponsView.setCollectionData();

      if (!isCrypto) {
        if (item.get('options').length) {
          // If we have options, we shouldn't be providing certain properties on the Item
          // model which track non-variant inventory
          item.unset('quantity');
          item.unset('productID');
          item.unset('infiniteInventory');
          delete formData.item.quantity;
          delete formData.item.productID;
          delete formData.item.infiniteInventory;

          // If we have options and are not tracking inventory, we'll set the infiniteInventory
          // flag for any skus.
          if (this.trackInventoryBy === 'DO_NOT_TRACK') {
            item.get('skus').forEach((sku) => {
              sku.unset('quantity');
              sku.set({ infiniteInventory: true });
            });
          }
        } else {
          formData.item.infiniteInventory = this.trackInventoryBy === 'DO_NOT_TRACK';

          if (this.trackInventoryBy === 'DO_NOT_TRACK') {
            formData.item.infiniteInventory = true;
            delete formData.item.quantity;
            item.unset('quantity');
          } else {
            formData.item.infiniteInventory = false;
          }
        }

        formData.metadata = {
          ...formData.metadata,
          format: 'FIXED_PRICE',
          acceptedCurrencies: this.$refs.cryptoCurSelector ? this.$refs.cryptoCurSelector.getState().activeCurs : metadata.get('acceptedCurrencies'),
        };
      } else {
        item.unset('condition');
        item.unset('productId');
        item.unset('price');
        metadata.unset('pricingCurrency');

        formData = {
          ...formData,
          coupons: [],
          item: {
            ...formData.item,
            options: [],
            skus: [],
          },
          metadata: {
            ...formData.metadata,
            acceptedCurrencies: typeof formData.metadata.acceptedCurrencies === 'string' ? [formData.metadata.acceptedCurrencies] : [],
            format: 'MARKET_PRICE',
          },
          shippingOptions: [],
        };
      }

      this.model.set({
        ...formData,
        item: {
          ...formData.item,
          tags: formData.item.tags.length ? formData.item.tags.split(tagsDelimiter) : [],
          categories: formData.item.categories.length ? formData.item.categories.split(tagsDelimiter) : [],
        },
      });

      // If the type is not 'PHYSICAL_GOOD', we'll clear out any shipping options.
      if (metadata.get('contractType') !== 'PHYSICAL_GOOD') {
        this.model.get('shippingOptions').reset();
      } else {
        // If any shipping options have a type of 'LOCAL_PICKUP', we'll
        // clear out any services that may be there.
        this.model.get('shippingOptions').forEach((shipOpt) => {
          if (shipOpt.get('type') === 'LOCAL_PICKUP') {
            shipOpt.set('services', []);
          }
        });
      }
    },

    open() {
      if (!this.openedBefore) {
        this.openedBefore = true;
        let cur;

        try {
          cur = this._origModel.unparsedResponse.listing.metadata.pricingCurrency.code;
        } catch (e) {
          return this;
        }

        if (!this.model.isCrypto && !getCurrencyByCode(cur)) {
          const unsupportedCurrencyDialog = new UnsupportedCurrency({
            unsupportedCurrency: cur,
          })
            .render()
            .open();

          this.listenTo(unsupportedCurrencyDialog, 'close', () => {
            const response = JSON.parse(JSON.stringify(this._origModel.unparsedResponse));
            const newCur = unsupportedCurrencyDialog.getCurrency();
            setDeepValue(response, 'listing.metadata.pricingCurrency', newCur);
            this.model.set(this.model.parse(response));
            this.$currencySelect.val(newCur);
            this.render();
          });
        }
      }

      return this;
    },

    showMaxTagsWarning() {
      this.$maxTagsWarning.empty().append(this.maxTagsWarning);
    },

    hideMaxTagsWarning() {
      this.$maxTagsWarning.empty();
    },

    showMaxCatsWarning() {
      this.$maxCatsWarning.empty().append(this.maxCatsWarning);
    },

    hideMaxCatsWarning() {
      this.$maxCatsWarning.empty();
    },

    createShippingOptionView(opts) {
      const options = {
        getCurrency: () => (this.$currencySelect.length ? this.$currencySelect.val() : this.model.get('metadata').pricingCurrency),
        ...(opts || {}),
      };
      const view = this.createChild(ShippingOption, options);

      this.listenTo(view, 'click-remove', (e) => {
        this.shippingOptions.remove(this.shippingOptions.at(this.shippingOptionViews.indexOf(e.view)));
      });

      return view;
    },

    render() {
      const item = this.model.get('item');
      const metadata = this.model.get('metadata');

      this.currencies = this.currencies || getCurrenciesSortedByCode();

      this.setContractTypeClass(metadata.get('contractType'));

      this._$currencySelect = null;
      this._$priceInput = null;
      this._$buttonSave = null;
      this._$inputPhotoUpload = null;
      this._$photoUploadingLabel = null;
      this._$editListingReturnPolicy = null;
      this._$editListingTermsAndConditions = null;
      this._$sectionShipping = null;
      this._$maxCatsWarning = null;
      this._$maxTagsWarning = null;
      this._$addShipOptSectionHeading = null;
      this._$variantInventorySection = null;
      this._$itemPrice = null;
      this.$modalContent = $('.modalContent');
      this.$tabControls = $('.tabControls');
      this.$titleInput = $('#editListingTitle');
      this.$editListingTags = $('#editListingTags');
      this.$editListingCategories = $('#editListingCategories');
      this.$shippingOptionsWrap = $('.js-shippingOptionsWrap');
      this.$couponsSection = $('.js-couponsSection');
      this.$variantsSection = $('.js-variantsSection');

      $('#editContractType, #editListingVisibility, #editListingCondition, ' + '#editListingCountrySelect').select2({
        // disables the search box
        minimumResultsForSearch: Infinity,
      });

      $('#editListingCurrency')
        .select2({
          matcher: (params, data) => {
            if (!params.term || params.term.trim() === '') {
              return data;
            }

            const term = params.term.toUpperCase().trim();

            const name = data.element.getAttribute('data-name');

            if (data.text.toUpperCase().includes(term) || (name && name.toUpperCase().includes(term))) {
              return data;
            }

            return null;
          },
        })
        .on('change', () => this.variantInventory.render());

      this.$editListingTags.selectize({
        persist: false,
        maxItems: item.max.tags,
        create: (input) => {
          // we'll make the tag all lowercase and
          // replace spaces with dashes.
          const term = input
            .toLowerCase()
            .replace(/\s/g, '-')
            .replace('#', '')
            // replace consecutive dashes with one
            .replace(/-{2,}/g, '-');
          return {
            value: term,
            text: term,
          };
        },
        onChange: (value) => {
          const tags = value.length ? value.split(',') : [];
          if (tags.length >= item.max.tags) {
            this.showMaxTagsWarning();
          } else {
            this.hideMaxTagsWarning();
          }
        },
      });

      this.$editListingCategories.selectize({
        persist: false,
        maxItems: item.max.cats,
        create: (input) => ({
          value: input,
          text: input,
        }),
        onChange: (value) => {
          const cats = value.length ? value.split(',') : [];
          if (cats.length >= item.max.cats) {
            this.showMaxCatsWarning();
          } else {
            this.hideMaxCatsWarning();
          }
        },
      });

      // render shipping options
      this.shippingOptionViews.forEach((shipOptVw) => shipOptVw.remove());
      this.shippingOptionViews = [];
      const shipOptsFrag = document.createDocumentFragment();

      this.model.get('shippingOptions').forEach((shipOpt, shipOptIndex) => {
        const shipOptVw = this.createShippingOptionView({
          model: shipOpt,
          listPosition: shipOptIndex + 1,
        });

        this.shippingOptionViews.push(shipOptVw);
        shipOptVw.render().$el.appendTo(shipOptsFrag);
      });

      this.$shippingOptionsWrap.append(shipOptsFrag);

      // render sku field
      if (this.skuField) this.skuField.remove();

      this.skuField = this.createChild(SkuField, {
        model: item,
        initialState: {
          variantsPresent: !!item.get('options').length,
        },
      });

      $('.js-skuFieldContainer').html(this.skuField.render().el);

      // render variants
      if (this.variantsView) this.variantsView.remove();

      const variantErrors = {};

      Object.keys(item.validationError || {}).forEach((errKey) => {
        if (errKey.startsWith('options[')) {
          variantErrors[errKey] = item.validationError[errKey];
        }
      });

      this.variantsView = this.createChild(Variants, {
        collection: this.variantOptionsCl,
        maxVariantCount: item.max.optionCount,
        errors: variantErrors,
      });

      this.variantsView.listenTo(this.variantsView, 'variantChoiceChange', this.onVariantChoiceChange.bind(this));

      this.$variantsSection.find('.js-variantsContainer').append(this.variantsView.render().el);

      // render inventory management section
      if (this.inventoryManagement) this.inventoryManagement.remove();
      const inventoryManagementErrors = {};

      if (this.model.validationError && this.model.validationError['item.quantity']) {
        inventoryManagementErrors.quantity = this.model.validationError['item.quantity'];
      }

      this.inventoryManagement = this.createChild(InventoryManagement, {
        initialState: {
          trackBy: this.trackInventoryBy,
          quantity: item.get('quantity'),
          errors: inventoryManagementErrors,
        },
      });

      $('.js-inventoryManagementSection').html(this.inventoryManagement.render().el);
      this.listenTo(this.inventoryManagement, 'changeManagementType', this.onChangeManagementType);

      // render variant inventory
      if (this.variantInventory) this.variantInventory.remove();

      this.variantInventory = this.createChild(VariantInventory, {
        collection: item.get('skus'),
        optionsCl: item.get('options'),
        getPrice: () => this.getFormData(this.$itemPrice).item.price,
        getCurrency: () => this.currency,
      });

      $('.js-variantInventoryTableContainer').html(this.variantInventory.render().el);

      // render coupons
      if (this.couponsView) this.couponsView.remove();

      this.couponsView = this.createChild(Coupons, {
        collection: this.coupons,
        maxCouponCount: this.model.max.couponCount,
      });

      this.$couponsSection.find('.js-couponsContainer').append(this.couponsView.render().el);

      installRichEditor($('#editListingDescription'), {
        topLevelClass: 'clrBr',
      });

      if (this.sortablePhotos) this.sortablePhotos.destroy();
      this.sortablePhotos = Sortable.create(this.$refs.photoUploadItems, {
        filter: '.js-addPhotoWrap',
        onUpdate: (e) => {
          const imageModels = this.model.get('item').get('images').models;

          const movingModel = imageModels[e.oldIndex - 1];
          imageModels.splice(e.oldIndex - 1, 1);
          imageModels.splice(e.newIndex - 1, 0, movingModel);
        },
        onMove: (e) => ($(e.related).hasClass('js-addPhotoWrap') ? false : undefined),
      });

      if (this.cryptoCurrencyType) this.cryptoCurrencyType.remove();
      this.cryptoCurrencyType = this.createChild(CryptoCurrencyType, {
        model: this.model,
        getCoinTypes: this.getCoinTypesDeferred.promise(),
        getReceiveCur: () => this._receiveCryptoCur,
      });

      $('.js-cryptoTypeWrap').html(this.cryptoCurrencyType.render().el);

      setTimeout(() => {
        if (!this.rendered) {
          this.rendered = true;
          this.$titleInput.focus();
        }
      });

      // This block should be after any dom manipulation in render.
      if (this.createMode) {
        if (!this.attrsAtCreate) {
          this.setModelData();
          this.attrsAtCreate = this.model.toJSON();
        }
      } else if (!this.attrsAtLastSave) {
        this.setModelData();
        this.attrsAtLastSave = this.model.toJSON();
      }
      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
