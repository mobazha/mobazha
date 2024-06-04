<template>
  <div
    :class="`modal editListing tabbedModal modalScrollPage TYPE_${formData.metadata.contractType} ${!createMode ? 'editMode' : ''} ${
      fixedNav ? 'fixedNav' : ''
    } ${trackInventoryBy === 'DO_NOT_TRACK' ? 'notTrackingInventory' : ''}`"
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
              <a
                v-for="(tab, index) in tabs"
                :key="index"
                @click="scrollTo(tab.key)"
                :class="`tab row tab-${tab.key} ${activeTab === tab.key ? 'active' : ''}`"
                >{{ tab.name }}</a
              >
            </div>
          </div>
          <div class="flexExpand posR tabContent">
            <div class="gutterVMd2 js-formSectionsContainer">
              <section ref="sectionGeneral" class="generalSection contentBox padMd clrP clrBr clrSh3">
                <div class="flexHCent">
                  <h2 class="h3 clrT js-listingHeading">
                    {{ ob.createMode ? ob.polyT('editListing.createListingLabel') : ob.polyT('editListing.editListingLabel') }}
                  </h2>
                  <a :class="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph modalContentCornerBtn ${saving ? 'disable' : ''}`" @click="onSaveClick">{{
                    ob.polyT('settings.btnSave')
                  }}</a>
                </div>
                <hr class="clrBr" />

                <div class="tabFormWrapper">
                  <form class="box padSmKids padStack">
                    <div class="standardTypeWrap js-standardTypeWrap pad0 padSmKids padStackAll" v-if="formData.metadata.contractType !== 'CRYPTOCURRENCY'">
                      <div class="flexRow">
                        <div class="col12">
                          <div class="flexRow">
                            <label for="editListingTitle" class="required flexExpand">{{ ob.polyT('editListing.title') }}</label>
                            <ViewListingLinks :createMode="ob.createMode" @viewListing="onClickViewListing" @viewListingOnWeb="onClickViewListingOnWeb" />
                          </div>
                          <FormError v-if="ob.errors['item.title']" :errors="ob.errors['item.title']" />
                          <input
                            type="text"
                            v-focus
                            class="clrBr clrP clrSh2"
                            v-model.trim="formData.item.title"
                            id="editListingTitle"
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
                          <Select2
                            id="editContractType"
                            v-model="formData.metadata.contractType"
                            :options="{
                              // disables the search box
                              minimumResultsForSearch: Infinity,
                            }"
                            class="clrBr clrP clrSh2 marginTopAuto"
                            style="width: 100%"
                          >
                            <template v-for="(contractType, j) in ob.contractTypes" :key="j">
                              <option :value="contractType.code" :selected="contractType.code === formData.metadata.contractType">
                                {{ contractType.name }}
                              </option>
                            </template>
                          </Select2>
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperType', { smart_count: 4 }) }}</div>
                        </div>
                        <div class="col6 simpleFlexCol">
                          <!-- // hiding until this is ready on the back end -->
                          <div class="hide">
                            <label for="editListingVisibility" class="required">{{ ob.polyT('editListing.visibility') }}</label>
                            <Select2
                              id="editListingVisibility"
                              class="clrBr clrP clrSh2 marginTopAuto"
                              :options="{
                                // disables the search box
                                minimumResultsForSearch: Infinity,
                              }"
                            >
                              <option value="hidden">Hidden (doesn't display in store)</option>
                            </Select2>
                          </div>
                        </div>
                      </div>
                      <div class="flexRow gutterH">
                        <div class="col4 simpleFlexCol">
                          <label for="editListingPrice" class="required">{{ ob.polyT('editListing.price') }}</label>
                          <FormError v-if="ob.errors['item.price']" :errors="ob.errors['item.price']" />
                          <FormError v-if="ob.errors['metadata.pricingCurrency.code']" :errors="ob.errors['metadata.pricingCurrency.code']" />
                          <div class="inputSelect marginTopAuto">
                            <input
                              type="number"
                              class="clrBr clrP clrSh2"
                              v-model="formData.item.price"
                              id="editListingPrice"
                              placeholder="0.00"
                              data-var-type="bignumber"
                            />
                            <Select2 id="editListingCurrency" v-model="formData.metadata.pricingCurrency.code" class="clrBr clrP nestInputRight">
                              <template v-for="(currency, j) in currencies" :key="j">
                                <option :value="currency.code" :selected="currency.code.toUpperCase() === formData.metadata.pricingCurrency.code.toUpperCase()">
                                  {{ currency.code }}
                                </option>
                              </template>
                            </Select2>
                          </div>
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperPrice', { cur: helperCryptoCurName }) }}</div>
                        </div>
                        <div class="col4 simpleFlexCol conditionWrap">
                          <label for="editListingCondition" class="required">{{ ob.polyT('editListing.condition') }}</label>
                          <FormError v-if="ob.errors['item.condition']" :errors="ob.errors['item.condition']" />
                          <Select2
                            id="editListingCondition"
                            v-model="formData.item.condition"
                            class="clrBr clrP clrSh2 marginTopAuto"
                            style="width: 100%"
                            :options="{ minimumResultsForSearch: Infinity }"
                          >
                            <template v-for="(conditionType, j) in ob.conditionTypes" :key="j">
                              <option :value="conditionType.code" :selected="conditionType.code === formData.item.condition">{{ conditionType.name }}</option>
                            </template>
                          </Select2>
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperCondition') }}</div>
                        </div>
                        <div class="col3 simpleFlexCol weightWrap">
                          <label for="editListingWeight" class="required">{{ ob.polyT('editListing.weight') }} (g)</label>
                          <FormError v-if="ob.errors['item.grams']" :errors="ob.errors['item.grams']" />
                          <input type="number" class="clrBr clrP clrSh2 marginTopAuto" v-model="formData.item.grams" id="editListingWeight" :placeholder="0" />
                          <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperWeight') }}</div>
                        </div>
                      </div>
                    </div>
                    <div class="cryptoTypeWrap js-cryptoTypeWrap pad0" v-if="formData.metadata.contractType === 'CRYPTOCURRENCY'">
                      <CryptoCurrencyType
                        v-model="formData"
                        :options="{
                          getCoinTypes: getCoinTypesDeferred.promise(),
                          receiveCur,
                        }"
                        :bb="
                          function () {
                            return {
                              model,
                            };
                          }
                        "
                        @clickViewListing="onClickViewListing"
                        @clickViewListingOnWeb="onClickViewListingOnWeb"
                      />
                    </div>
                    <div class="flexRow gutterH skuMatureContentRow js-skuMatureContentRow">
                      <div class="col6 simpleFlexCol js-skuFieldContainer">
                        <div>
                          <label for="editListingSku">{{ ob.polyT('editListing.sku') }}</label>
                          <FormError v-if="ob.errors['item.productId']" :errors="ob.errors['item.productId']" />
                          <input
                            type="text"
                            class="clrBr clrP clrSh2 marginTopAuto"
                            :disabled="showVariantInventorySection"
                            v-model="formData.item.productID"
                            id="editListingSku"
                            :placeholder="ob.polyT('editListing.placeholderSKU')"
                            :maxlength="ob.max.productIdLength"
                          />
                          <div
                            class="clrT2 txSm helper"
                            @click="onClickScrollToVariantInventory"
                            v-html="
                              showVariantInventorySection
                                ? ob.polyT('editListing.helperSKUWithVariants', {
                                    variantInventoryLink: `<a id=&quot;scrollToVariantInventory&quot;>${ob.polyT('editListing.variantInventoryLink')}</a>`,
                                  })
                                : ob.polyT('editListing.helperSKU')
                            "
                          ></div>
                        </div>
                      </div>
                      <div class="col6 simpleFlexCol">
                        <label>{{ ob.polyT('editListing.nsfwLabel') }}</label>
                        <FormError v-if="ob.errors['item.nsfw']" :errors="ob.errors['item.nsfw']" />
                        <div class="btnStrip">
                          <div class="btnRadio clrBr">
                            <input type="radio" v-model="formData.item.nsfw" value="true" id="editListingNSFWInputTrue" />
                            <label for="editListingNSFWInputTrue">{{ ob.polyT('editListing.nsfwYes') }}</label>
                          </div>
                          <div class="btnRadio clrBr">
                            <input type="radio" v-model="formData.item.nsfw" value="false" id="editListingNSFWInputFalse" />
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
                        <Tinymce
                          class="clrBr clrSh2"
                          id="editListingDescription"
                          v-model="formData.item.description"
                          :height="500"
                          :placeholder="ob.polyT('editListing.placeholderDescription')"
                        ></Tinymce>
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
                <input type="file" id="inputPhotoUpload" ref="inputPhotoUpload" @change="onChangePhotoUploadInput" accept="image/*" class="hide" multiple />
                <ul ref="photoUploadItems" class="unstyled uploadItems clrBr rowSm js-photoUploadItems">
                  <li class="addElement tile js-addPhotoWrap">
                    <span class="imagesIcon ion-images clrT4"></span>
                    <button class="btn clrP clrBr clrT tx6" @click="$refs.inputPhotoUpload.click()">{{ ob.polyT('editListing.btnAddPhoto') }}</button>
                  </li>
                  <template v-for="(image, j) in images.toJSON()" :key="image.id">
                    <UploadPhoto :image="image" @closeIcon="onClickRemoveImage(j)" />
                  </template>
                </ul>
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperPhotos', { maxPhotos: ob.max.photos }) }}</div>
              </section>

              <section ref="sectionIntroVideo" class="photosSection photoUploadSection contentBox padMd clrP clrBr clrSh3 tx3">
                <div class="overflowAuto">
                  <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.introVideo') }}</h2>
                  <div class="js-videoUploadingLabel floR" v-show="!!ob.videoUploadInprogress">
                    {{ ob.polyT('editListing.uploading') }} <a class="" @click="onClickCancelVideoUploads">{{ ob.polyT('editListing.btnCancelUpload') }}</a>
                  </div>
                  <hr class="clrBr rowMd" />
                </div>
                <FormError v-if="ob.errors['item.introVideo']" :errors="ob.errors['item.introVideo']" />
                <input type="file" id="introVideoUpload" ref="introVideoUpload" @change="onIntroVideoUploadInput" accept="video/mp4" class="hide" />
                <ul ref="introVideoUploadItems" class="unstyled uploadItems clrBr rowSm js-introVideoUploadItems">
                  <li class="addElement tile js-addIntroVideoWrap">
                    <span class="imagesIcon ion-images clrT4"></span>
                    <button class="btn clrP clrBr clrT tx6" @click="$refs.introVideoUpload.click()">{{ ob.polyT('editListing.btnAddPhoto') }}</button>
                  </li>
                  <li v-if="formData.item.introVideo" class="tile">
                    <video-player-item
                      :key="formData.item.introVideo.hash"
                      class="videoIntro floR clrT4"
                      :url="app.getServerUrl(`ob/file/${formData.item.introVideo.hash}`)"
                    />
                    <a class="closeIcon tx2" @click="onRemoveIntroVideo">
                      <span class="ion-ios-close-empty clrBr clrP clrT"></span>
                    </a>
                  </li>
                </ul>
              </section>

              <section ref="sectionTags" class="tagsSection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.tagsDetailed') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['item.tags']" :errors="ob.errors['item.tags']" />
                <div class="js-maxTagsWarning">
                  <div v-if="formData.item.tags.length >= ob.max.tags" class="clrT2 tx5 row">{{ ob.polyT('editListing.maxTagsWarning') }}</div>
                </div>
                <input
                  type="text"
                  ref="editListingTags"
                  id="editListingTags"
                  @input="(event) => (formData.item.tags = event.target.value.length ? event.target.value.split(ob.tagsDelimiter) : [])"
                  class="clrBr clrP hashPrefacedTags hideDropDown"
                  :value="formData.item.tags.join(ob.tagsDelimiter)"
                  :placeholder="ob.polyT('editListing.tagsPlaceholder')"
                />
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperTags') }}</div>
              </section>

              <section ref="sectionCategory" class="categorySection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.categoryDetailed') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['item.categories']" :errors="ob.errors['item.categories']" />
                <div class="js-maxCatsWarning">
                  <div v-if="ob.item.categories.length >= ob.max.cats" class="clrT2 tx5 row">{{ ob.polyT('editListing.maxCatsWarning') }}</div>
                </div>
                <input
                  type="text"
                  ref="editListingCategories"
                  id="editListingCategories"
                  @input="(event) => (formData.item.categories = event.target.value.length ? event.target.value.split(ob.tagsDelimiter) : [])"
                  class="clrBr clrP hideDropDown"
                  :value="formData.item.categories.join(ob.tagsDelimiter)"
                  :placeholder="ob.polyT('editListing.categoryPlaceholder')"
                />
              </section>

              <section
                ref="sectionVariants"
                :class="`variantsSection js-variantsSection contentBox padMd clrP clrBr clrSh3 tx3 ${
                  showVariantInventorySection ? 'expandedVariantsView' : ''
                }`"
              >
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.variantsDetailed') }}</h2>
                <hr class="clrBr rowMd" />
                <div class="js-variantsContainer variantsContainer">
                  <Variants
                    ref="variantsView"
                    :options="{
                      maxVariantCount: ob.max.optionCount,
                      errors: variantErrors,
                    }"
                    :bb="
                      function () {
                        return {
                          collection: variantOptionsCl,
                        };
                      }
                    "
                    @update="onUpdateVariantOptions"
                  />
                </div>
                <a class="btn clrP clrBr clrSh2 addFirstVariant" @click="onClickAddFirstVariant">{{ ob.polyT('editListing.variants.btnAddVariant') }}</a>
              </section>

              <section class="contentBox padMd clrP clrBr clrSh3 tx3 js-inventoryManagementSection inventoryManagementSection">
                <InventoryManagement
                  :key="trackInventoryBy"
                  :options="{
                    trackBy: trackInventoryBy,
                    quantity: formData.item.quantity,
                    errors: ob.errors['item'] || {},
                  }"
                  @changeManagementType="onChangeManagementType"
                  @changeInventoryQuantity="onChangeInventoryQuantity"
                />
              </section>

              <section
                ref="sectionVariantInventory"
                class="contentBox variantInventorySection js-variantInventorySection padMd clrP clrBr clrSh3 tx3"
                v-show="showVariantInventorySection"
              >
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.variantInventory') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['item.skus']" :errors="ob.errors['item.skus']" />
                <div class="js-variantInventoryTableContainer">
                  <VariantInventory
                    ref="variantInventory"
                    :key="`${formData.item.price}_${formData.metadata.pricingCurrency.code}_${variantOptionsKey}`"
                    :options="{
                      basePrice: formData.item.price,
                      listingCurrency: formData.metadata.pricingCurrency.code,
                    }"
                    :bb="
                      function () {
                        return {
                          collection: model.get('item').get('skus'),
                          optionsCl: variantOptionsCl,
                        };
                      }
                    "
                  />
                </div>
              </section>
              <section ref="optionalFeatures" class="contentBox optionalFeatures padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">Optional Features</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['item.skus']" :errors="ob.errors['item.skus']" />
                <div class="js-variantInventoryTableContainer">
                  <OptionalFeatures
                    ref="optionalFeatures"
                    :key="`${formData.item.price}_${formData.metadata.pricingCurrency.code}_${variantOptionsKey}`"
                    :options="{
                      basePrice: formData.item.price,
                      listingCurrency: formData.metadata.pricingCurrency.code,
                    }"
                    :bb="
                      function () {
                        return {
                          collection: model.get('item').get('skus'),
                          optionsCl: variantOptionsCl,
                        };
                      }
                    "
                  />
                </div>
              </section>
              <section ref="sectionReturnPolicy" class="returnPolicySection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.returnPolicy') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['refundPolicy']" :errors="ob.errors['refundPolicy']" />
                <a class="btn clrP clrBr clrSh2 rowSm %>" v-show="!ob.expandedReturnPolicy" @click="onClickAddReturnPolicy">{{
                  ob.polyT('editListing.btnAddReturnPolicy')
                }}</a>
                <textarea
                  ref="returnPolicy"
                  rows="8"
                  v-model="formData.refundPolicy"
                  class="clrBr clrP clrSh2"
                  v-show="ob.expandedReturnPolicy"
                  :placeholder="ob.polyT('editListing.placeholderReturnPolicy')"
                ></textarea>
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperReturnPolicy') }}</div>
              </section>

              <section ref="sectionTermsAndConditions" class="termsAndConditionsSection contentBox padMd clrP clrBr clrSh3 tx3">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.termsAndConditions') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['termsAndConditions']" :errors="ob.errors['termsAndConditions']" />
                <a class="btn clrP clrBr clrSh2 rowSm" v-show="!ob.expandedTermsAndConditions" @click="onClickAddTermsAndConditions">{{
                  ob.polyT('editListing.btnTermsAndConditions')
                }}</a>
                <textarea
                  ref="termsAndConditions"
                  rows="8"
                  v-model="formData.termsAndConditions"
                  class="clrBr clrP clrSh2"
                  v-show="ob.expandedTermsAndConditions"
                  :placeholder="ob.polyT('editListing.placeholderTerms')"
                ></textarea>
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperTerms') }}</div>
              </section>

              <section
                ref="sectionCoupons"
                :class="`couponsSection contentBox padMd clrP clrBr clrSh3 tx3 js-couponsSection ${coupons.length ? 'expandedCouponView' : ''}`"
              >
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.coupons') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError v-if="ob.errors['coupons']" :errors="ob.errors['coupons']" />
                <div class="js-couponsContainer couponsContainer">
                  <Coupons
                    ref="couponsView"
                    :options="{ maxCouponCount: model.max.couponCount }"
                    :bb="
                      function () {
                        return {
                          collection: coupons,
                        };
                      }
                    "
                  />
                </div>
                <a class="btn clrP clrBr clrSh2 btnAddCoupon" @click="onClickAddCoupon">{{ ob.polyT('editListing.btnAddCoupon') }}</a>
              </section>

              <section ref="sectionAcceptedCurs" class="acceptedCursSection contentBox padMd clrP clrBr clrSh3 tx3 acceptedCurrenciesSection">
                <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.acceptedCurrencies') }}</h2>
                <hr class="clrBr rowMd" />
                <FormError
                  v-if="ob.errors['metadata.acceptedCurrencies'] && formData.metadata.contractType !== 'CRYPTOCURRENCY'"
                  :errors="ob.errors['metadata.acceptedCurrencies']"
                />
                <div class="js-cryptoCurSelectContainer rowSm">
                  <CryptoCurSelector
                    :options="{
                      currencies: [...supportedWalletCurs()],
                      sort: true,
                    }"
                    v-model:activeCurs="formData.metadata.acceptedCurrencies"
                  />
                </div>
                <div class="clrT2 txSm helper">{{ ob.polyT('editListing.helperAcceptedCurrencies') }}</div>
              </section>

              <div class="contentBox padMd clrP clrBr clrSh3">
                <div class="flexHRight flexVCent gutterH">
                  <ViewListingLinks :createMode="ob.createMode" @viewListing="onClickViewListing" @viewListingOnWeb="onClickViewListingOnWeb" />
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

import Sortable from 'sortablejs';
import _ from 'underscore';
import path from 'path';
import Backbone from 'backbone';
import bigNumber from 'bignumber.js';
import 'velocity-animate/velocity.ui';
import app from '../../../../backbone/app';
import { isScrolledIntoView, openExternal } from '../../../../backbone/utils/dom';
import { startAjaxEvent, endAjaxEvent } from '../../../../backbone/utils/metrics';
import { getCurrenciesSortedByCode, getCurrencyByCode } from '../../../../backbone/data/currencies';
import {
  getCurrenciesSortedByName as getCryptoCursByName,
  getCurrenciesSortedByCode as getCryptoCursByCode,
} from '../../../../backbone/data/cryptoListingCurrencies';
import { supportedWalletCurs } from '../../../../backbone/data/walletCurrencies';
import { setDeepValue } from '../../../../backbone/utils/object';
import Image from '../../../../backbone/models/listing/Image';
import Coupon from '../../../../backbone/models/listing/Coupon';
import VariantOption from '../../../../backbone/models/listing/VariantOption';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
import { capitalize } from '../../../../backbone/utils/string';
import { toStandardNotation } from '../../../../backbone/utils/number';
import SimpleMessage, { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import Dialog from '../../../../backbone/views/modals/Dialog';
import UnsupportedCurrency from '../../../../backbone/views/modals/editListing/UnsupportedCurrency';

import ViewListingLinks from './ViewListingLinks.vue';
import UploadPhoto from './UploadPhoto.vue';
import CryptoCurrencyType from './CryptoCurrencyType.vue';
import Variants from './Variants.vue';
import InventoryManagement from './InventoryManagement.vue';
import VariantInventory from './VariantInventory.vue';
import OptionalFeatures from './OptionalFeatures.vue';

import Coupons from './Coupons.vue';

import Tinymce from './../../../components/Tinymce/index.vue';

export default {
  components: {
    ViewListingLinks,
    CryptoCurrencyType,
    UploadPhoto,
    Variants,
    InventoryManagement,
    VariantInventory,
    OptionalFeatures,
    Coupons,
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
      app,

      activeTab: 'general',

      fixedNav: false,

      images: undefined,
      photoUploadsKey: 0,

      videoUploadsKey: 0,

      currencies: [],
      expandedReturnPolicy: false,
      expandedTermsAndConditions: false,

      getCoinTypesDeferred: $.Deferred(),
      variantOptionsCl: [],
      variantOptionsKey: 0,
      coupons: [],

      formData: {
        item: {
          title: '',
          price: 0,
          condition: '',
          introVideo: undefined,
          productID: '',
          nsfw: true,
          description: '',
          tags: [],
          categories: [],
        },
        metadata: {
          contractType: '',
          pricingCurrency: {
            code: '',
          },
        },
        refundPolicy: '',
        termsAndConditions: '',
      },
      trackInventoryBy: '',
      saving: false,
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

    this.inProgressVideoUploads.forEach((upload) => upload.abort());
  },
  computed: {
    ob() {
      const item = this.model.get('item');
      const metadata = this.model.get('metadata');

      return {
        ...this.templateHelpers,
        app,
        createMode: this.createMode,
        returnText: this.options.returnText,
        countryList: this.countryList,
        contractTypes: metadata.contractTypesVerbose,
        conditionTypes: this.model.get('item').conditionTypes.map((conditionType) => ({
          code: conditionType,
          name: app.polyglot.t(`conditionTypes.${conditionType}`),
        })),
        errors: this.model.validationError || {},
        photoUploadInprogress: !!this.inProgressPhotoUploads.length,
        videoUploadInprogress: !!this.inProgressVideoUploads.length,
        expandedReturnPolicy: this.expandedReturnPolicy || !!this.formData.refundPolicy,
        expandedTermsAndConditions: this.expandedTermsAndConditions || !!this.formData.termsAndConditions,
        max: {
          title: item.max.titleLength,
          cats: item.max.cats,
          tags: item.max.tags,
          productIdLength: item.max.productIdLength,
          photos: this.MAX_PHOTOS,
          optionCount: item.max.optionCount,
        },
        ...this.model.toJSON(),
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

    showVariantInventorySection() {
      return !!this.variantOptionsCl.length;
    },

    inProgressPhotoUploads() {
      let access = this.photoUploadsKey;

      return this.photoUploads.filter((upload) => upload.state() === 'pending');
    },

    inProgressVideoUploads() {
      let access = this.videoUploadsKey;

      return this.videoUploads.filter((upload) => upload.state() === 'pending');
    },

    receiveCur() {
      const acceptedCurs = this.model.get('metadata').get('acceptedCurrencies');
      return this.model.isCrypto ? (acceptedCurs.length && acceptedCurs()[0]) || null : null;
    },

    variantErrors() {
      const variantErrors = {};

      const item = this.model.get('item');
      Object.keys(item.validationError || {}).forEach((errKey) => {
        if (errKey.startsWith('options[')) {
          variantErrors[errKey] = item.validationError[errKey];
        }
      });
    },
  },
  methods: {
    supportedWalletCurs,
    initFormData() {
      const model = this.model.toJSON();

      let cur = app.settings.get('localCurrency');
      try {
        cur = model.metadata.pricingCurrency.code;
      } catch (e) {
        // pass
      }

      this.formData = {
        item: {
          title: model.item.title,
          price: toStandardNotation(model.item.price),
          condition: model.item.condition,
          grams: model.item.grams,
          introVideo: model.item.introVideo,
          productID: model.item.productID,
          nsfw: model.item.nsfw,
          tags: model.item.tags,
          categories: model.item.categories,
          description: model.item.description,
          quantity: model.item.quantity,
        },
        metadata: {
          contractType: model.metadata.contractType,
          pricingCurrency: {
            code: cur,
          },
          acceptedCurrencies: model.metadata.acceptedCurrencies,
        },
        refundPolicy: model.refundPolicy,
        termsAndConditions: model.termsAndConditions,
      };

      const item = this.model.get('item');
      if (item.isInventoryTracked) {
        this.trackInventoryBy = item.get('options').length ? 'TRACK_BY_VARIANT' : 'TRACK_BY_FIXED';
      } else {
        this.trackInventoryBy = 'DO_NOT_TRACK';
      }
    },
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

      this.currencies = getCurrenciesSortedByCode();

      this.initFormData();

      this.listenTo(this.model, 'sync', () => {
        setTimeout(() => {
          if (this.createMode && !this.model.isNew()) {
            this.createMode = false;
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
      this.videoUploads = [];
      this.countryList = getTranslatedCountries();

      getCryptoCursByName().then(
        (curs) => this.getCoinTypesDeferred.resolve(curs),
        () => this.getCoinTypesDeferred.resolve(getCryptoCursByCode().map((cur) => ({ code: cur, name: cur })))
      );

      this.coupons = this.model.get('coupons');

      this.variantOptionsCl = this.model.get('item').get('options');

      this.listenTo(this.variantOptionsCl, 'update', this.onUpdateVariantOptions);
    },

    onClickReturn() {
      this.$emit('click-return', { view: this });
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
      this.photoUploadsKey += 1;
    },

    onClickCancelVideoUploads() {
      this.inProgressVideoUploads.forEach((videoUpload) => videoUpload.abort());
      this.videoUploadsKey += 1;
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
      let photoFiles = Array.prototype.slice.call(this.$refs.inputPhotoUpload.files, 0);

      // prune out any non-image files
      photoFiles = photoFiles.filter((file) => file.type.startsWith('image'));

      this.$refs.inputPhotoUpload.value = '';

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

      const toUpload = [];
      let loaded = 0;
      let errored = 0;

      photoFiles.forEach((photoFile) => {
        const newImage = document.createElement('img');

        newImage.src = URL.createObjectURL(photoFile);

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

    onIntroVideoUploadInput() {
      var formData = new FormData();
      var files = this.$refs.introVideoUpload.files[0];
      formData.append('file', files);
      formData.append('type', 'introVideo');

      this.$refs.introVideoUpload.value = '';

      const upload = $.ajax({
        url: app.getServerUrl('ob/file'),
        type: 'POST',
        data: formData,
        processData: false, // tell jQuery not to process the data
        contentType: false, // tell jQuery not to set contentType
      })
        .done((uploadedFile) => {
          if (this.isRemoved()) return;

          this.formData.item.introVideo = { filename: uploadedFile.name, hash: uploadedFile.hash, type: 'video' };
        })
        .fail((jqXhr) => {
          openSimpleMessage(app.polyglot.t('editListing.errors.uploadVideoErrorTitle'), (jqXhr.responseJSON && jqXhr.responseJSON.reason) || '');
        })
        .always(() => {
          this.videoUploadsKey += 1;
        });

      this.videoUploads.push(upload);
      this.videoUploadsKey += 1;
    },

    onRemoveIntroVideo() {
      this.formData.item.introVideo = undefined;
    },

    onClickAddReturnPolicy() {
      this.expandedReturnPolicy = true;
      this.$nextTick(() => {
        this.$refs.returnPolicy.focus();
      });
    },

    onClickAddTermsAndConditions() {
      this.expandedTermsAndConditions = true;
      this.$nextTick(() => {
        this.$refs.termsAndConditions.focus();
      });
    },

    onClickAddCoupon() {
      this.coupons.add(new Coupon());

      if (this.coupons.length === 1) {
        this.$nextTick(() => {
          $(this.$refs.sectionCoupons).find('.coupon input[name=title]').focus();
        });
      }
    },

    onClickAddFirstVariant() {
      this.variantOptionsCl.add(new VariantOption());

      if (this.variantOptionsCl.length === 1) {
        this.$nextTick(() => {
          $(this.$refs.sectionVariants).find('.variant input[name=name]').focus();
        });
      }
    },

    onUpdateVariantOptions() {
      this.variantOptionsKey += 1;

      if (this.showVariantInventorySection) {
        if (this.trackInventoryBy !== 'DO_NOT_TRACK') {
          this.trackInventoryBy = 'TRACK_BY_VARIANT';
        }
      } else {
        if (this.trackInventoryBy !== 'DO_NOT_TRACK') {
          this.trackInventoryBy = 'TRACK_BY_FIXED';
        }
      }
    },

    onClickScrollToVariantInventory(e) {
      if (e.target.id === 'scrollToVariantInventory') {
        this.scrollTo('variantInventory');
      }
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
        url: app.getServerUrl('ob/productimages'),
        type: 'POST',
        data: JSON.stringify(imagesToUpload),
        dataType: 'json',
        contentType: 'application/json',
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
          this.photoUploadsKey += 1;
        });

      this.photoUploads.push(upload);
      this.photoUploadsKey += 1;
    },

    scrollTo(tab) {
      this.$scrollTo(`.${tab}Section`, 300, {
        container: '.tabbedModal', //设置滚动容器
        easing: 'ease-in', //动画效果
        onDone: () => {
          setTimeout(() => {
            this.activeTab = tab;
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
      this.saving = true;
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
          .always(() => (this.saving = false))
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
        this.saving = false;
      }

      if (!save) {
        const firstErr = $('.errorList:visible').eq(0);
        if (firstErr.length) {
          firstErr[0].scrollIntoViewIfNeeded();
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

    onChangeManagementType(type) {
      if (type === 'TRACK') {
        this.trackInventoryBy = this.showVariantInventorySection ? 'TRACK_BY_VARIANT' : 'TRACK_BY_FIXED';
      } else {
        this.trackInventoryBy = 'DO_NOT_TRACK';
      }
    },

    onChangeInventoryQuantity(quantity) {
      this.formData.item.quantity = quantity;
    },

    /**
     * Will set the model with data from the form, including setting nested models
     * and collections which are managed by nested views.
     */
    setModelData() {
      let formData = this.formData;
      if (formData.item.price != null) {
        formData.item.price = bigNumber(formData.item.price);
      }

      const item = this.model.get('item');
      const metadata = this.model.get('metadata');
      const isCrypto = this.formData.metadata.contractType === 'CRYPTOCURRENCY';

      // set model / collection data for various child views
      this.$refs.variantsView.setCollectionData();
      this.$refs.variantInventory.setCollectionData();
      this.$refs.couponsView.setCollectionData();

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
            format: 'MARKET_PRICE',
          },
        };
      }

      this.formData = formData;
      this.model.set({
        ...formData,
        item: {
          ...formData.item,
        },
      });
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

            this.formData.metadata.pricingCurrency.code = newCur;
          });
        }
      }

      return this;
    },

    render() {
      const item = this.model.get('item');

      this.editListingTags = $(this.$refs.editListingTags).selectize({
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
        onChange: () => {
          this.formData.item.tags = this.editListingTags[0].selectize.items;
        },
      });

      this.editListingCategories = $(this.$refs.editListingCategories).selectize({
        persist: false,
        maxItems: item.max.cats,
        create: (input) => ({
          value: input,
          text: input,
        }),
        onChange: () => {
          this.formData.item.categories = this.editListingCategories[0].selectize.items;
        },
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
<style lang="scss" scoped>
::v-deep(.tox-fullscreen) {
  top: 50px !important;
}

.videoIntro {
  width: 102px;
  height: 102px;
}
</style>
