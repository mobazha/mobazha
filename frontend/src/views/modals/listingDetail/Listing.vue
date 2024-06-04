<template>
  <div>
    <div v-if="!showNsfwWarning && showModal" class="modal listingDetail modalScrollPage" @click="onDocumentClick">
      <BaseModal @close="close">
        <template v-slot:component>
          <div ref="popInMessages" class="popInMessageHolder js-popInMessages"></div>

          <div class="topControls withEndBtn flex">
            <template v-if="vendor">
              <div class="contentBox clrP clrSh3 clrBr clrT">
                <div class="padSm gutterHSm overflowAuto margRSm flexVCent">
                  <a class="clrBr2 clrSh1 disc storeOwnerAvatar flexNoShrink js-storeOwnerAvatar" :style="ob.getAvatarBgImage(vendor.avatarHashes)"></a>
                  <p class="txUnl tx3 clamp">{{ vendor.name }}</p>
                  <a class="link flexNoShrink tx6" @click="onClickGoToStore">{{
                    ob.openedFromStore ? ob.polyT('listingDetail.returnToStore') : ob.polyT('listingDetail.goToStore')
                  }}</a>
                </div>
              </div>
            </template>
            <template v-if="ob.ownListing">
              <div class="flexNoShrink" style="margin-left: auto">
                <div class="btnStrip clrSh3">
                  <button class="btn clrP clrBr" @click="onClickEditListing">{{ ob.polyT('listingDetail.edit') }}</button>
                  <button class="btn clrP clrBr" @click="onClickCloneListing">{{ ob.polyT('listingDetail.clone') }}</button>
                  <ProcessingButton
                    :className="`btn js-deleteListing clrP clrBr ${isDeleting ? 'processing' : ''}`"
                    @click.stop="onClickDeleteListing"
                    :btnText="ob.polyT('listingDetail.delete')"
                  />
                </div>
              </div>
              <div class="js-deleteConfirmedBox confirmBox deleteConfirm tx5 arrowBoxTop clrBr clrP clrT" v-show="showDeleteConfirmedBox" @click.stop.prevent>
                <div class="tx3 txB rowSm">{{ ob.polyT('listingDetail.confirmDelete.title') }}</div>
                <p>{{ ob.polyT('listingDetail.confirmDelete.body') }}</p>
                <hr class="clrBr row" />
                <div class="flexHRight flexVCent gutterHLg buttonBar">
                  <a class="" @click.stop="onClickConfirmCancel">{{ ob.polyT('listingDetail.confirmDelete.btnCancel') }}</a>
                  <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click.stop="onClickConfirmedDelete">{{
                    ob.polyT('listingDetail.confirmDelete.btnConfirm')
                  }}</a>
                </div>
              </div>
            </template>

            <template v-else>
              <div class="flexNoShrink" style="margin-left: auto">
                <div class="js-socialBtns">
                  <SocialBtns :options="{ targetID: vendor.peerID }" />
                </div>
              </div>
            </template>
          </div>

          <div class="listingContent flexColRow gutterVMd2">
            <div class="contentBox padLg clrP clrBr clrSh3">
              <div :class="`${ob.metadata.contractType !== 'CRYPTOCURRENCY' ? 'flex' : 'flexVCent'} gutterHLg`">
                <template v-if="ob.metadata.contractType !== 'CRYPTOCURRENCY'">
                  <h2 class="txUnb flexExpand">{{ ob.item.title }}</h2>
                  <h2 class="txUnb flexNoShrink js-price" v-html="totalPriceInfo"></h2>
                </template>

                <template v-else>
                  <h2 class="flexExpand js-cryptoTitle cryptoTitle">
                    <CryptoTradingPairWrap :options="cryptoTradingPairOptions" />
                  </h2>
                  <CryptoPrice
                    :options="{
                      priceAmount: ob.price.amount,
                      priceCurrencyCode: ob.price.currencyCode,
                      displayCurrency: ob.displayCurrency,
                      priceModifier: ob.price.modifier,
                      wrappingTag: 'h2',
                      wrappingClass: 'flexNoShrink txRgt tx3',
                    }"
                  />
                </template>
              </div>
              <div class="flex gutterHLg">
                <div class="mainImageWrapper">
                  <div
                    v-if="!ob.item.introVideo"
                    class="mainImage clrBr"
                    @click="onClickGotoPhotos"
                    :style="
                      ob.item.images.length
                        ? `background-image: url(${ob.getServerUrl(
                            `ob/image/${ob.isHiRez() ? ob.item.images[0].large : ob.item.images[0].medium}`
                          )}), url('../imgs/defaultItem.png')`
                        : `background-image: url('../imgs/defaultItem.png')`
                    "
                  ></div>
                  <video-player-item v-if="ob.item.introVideo" class="mainImage clrBr" :url="app.getServerUrl(`ob/file/${ob.item.introVideo.hash}`)" />
                  <div class="txCtr">
                    <a class="tx5" @click="onClickGotoPhotos">
                      <u>{{
                        ob.polyT('listingDetail.viewPhotos', {
                          count: ob.item.images.length,
                          smart_count: ob.item.images.length,
                        })
                      }}</u>
                    </a>
                  </div>
                </div>
                <div class="flexExpand">
                  <div class="buyBox clrP clrBr">
                    <div class="flexColRows flexHCent gutterV">
                      <template v-for="(item, optionIndex) in ob.item.options" :key="item.name">
                        <div class="flexVCent gutterHLg">
                          <div class="col4 h5 txUnl">{{ item.name }}</div>
                          <div class="col8 txLft">
                            <Select2 class="js-variantSelect" v-model="variantOptions[optionIndex]" @change="onChangeVariantSelect" :name="item.name">
                              <template v-for="variant in item.variants" :key="variant.name">
                                <option :value="variant.name">{{ variant.name }}</option>
                              </template>
                            </Select2>
                          </div>
                        </div>
                      </template>
                      <!-- <div class="flex">
                        <button
                          :class="`btnHg clrBAttGrad clrBrDec1 clrTOnEmph js-purchaseBtn flex-1 ${templateOptions.buyNowClass} ${
                            outdateHash ? 'disabled' : ''
                          }`"
                          @click="startPurchase"
                        >
                          {{ ob.polyT(templateOptions.buyNowTranslationKey) }}
                        </button>
                        <button :class="`btnHg clrBAttGrad clrBrDec1 clrTOnEmph js-addToCartBtn  flex-1 ${templateOptions.buyNowClass}`" @click="addToCart">
                          {{ ob.polyT('listingDetail.addToCart') }}
                        </button>
                      </div> -->
                      <div class="flex">
                        <button :class="`warning-btn flex-1 ${templateOptions.buyNowClass}`" type="success" round @click="addToCart">
                          {{ ob.polyT('listingDetail.addToCart') }}
                        </button>
                        <button :class="`success-btn flex-1 ${templateOptions.buyNowClass} ${outdateHash ? 'disabled' : ''}`" @click="startPurchase">
                          {{ ob.polyT(templateOptions.buyNowTranslationKey) }}
                        </button>
                      </div>
                      <div class="js-purchaseErrorWrap">
                        <template v-if="outdateHash">
                          <PurchaseError
                            @click.stop="onClickReloadOutdated"
                            :tip="
                              ob.polyT('listingDetail.errors.outdatedHash', {
                                reloadLink: `<a class=&quot;js-reloadOutdated&quot; id=&quot;reloadOutdated&quot;>${ob.polyT(
                                  'listingDetail.errors.reloadOutdatedHash'
                                )}<a>`,
                              })
                            "
                          ></PurchaseError>
                        </template>
                        <template v-else-if="templateOptions.unpurchaseable">
                          <PurchaseError :tip="templateOptions.tip"></PurchaseError>
                        </template>
                      </div>
                      <div class="flexHCent gutterH">
                        <div class="tx6 js-rating rating" @click="clickRating">
                          <Rating :options="ratingData" />
                        </div>
                        <template v-if="ob.shipsFreeToMe">
                          <div class="txCtr">
                            <a class="clrE1 clrTOnEmph phraseBox txNoUnd" @click="onClickFreeShippingLabel">{{
                              ob.polyT('listingDetail.freeShippingBanner')
                            }}</a>
                          </div>
                        </template>
                      </div>
                    </div>
                  </div>
                  <div class="flexHCent gutterHLg tx5 rowLg">
                    <div
                      v-html="
                        ob.polyT('listingDetail.type', {
                          type: `<b>${ob.polyT(`formats.${ob.metadata.contractType}`)}</b>`,
                        })
                      "
                    ></div>
                    <!-- // not showing the inventory for now since it's broken on the server -->
                    <template v-if="ob.isCrypto && false">
                      <div
                        v-html="
                          ob.polyT('listingDetail.inventory', {
                            inventory: `<span class=&quot;js-cryptoInventory&quot;></span>`,
                          })
                        "
                      ></div>
                    </template>
                    <template v-else-if="ob.metadata.contractType === 'PHYSICAL_GOOD'">
                      <div
                        v-html="
                          ob.polyT('listingDetail.condition', {
                            condition: `<b>${ob.polyT(`conditionTypes.${ob.item.condition.toUpperCase()}`, { _: ob.item.condition })}</b>`,
                          })
                        "
                      ></div>
                      <div v-html="ob.polyT('listingDetail.weight', { weight: `<b>${ob.item.grams ? ob.item.grams : 0}</b>` })"></div>
                    </template>
                  </div>
                  <hr class="rowLg" />
                  <table class="table">
                    <tr>
                      <th><input type="checkbox" @change="changeCheckAll" :checked="isCheckAll" /></th>
                      <th>Name</th>
                      <th>Surcharge</th>
                      <th>SKU</th>
                      <th>Image</th>
                    </tr>
                    <tr>
                      <td>
                        <input type="checkbox" name="checked" :value="1" v-model="checkBoxValue" />
                      </td>
                      <td>Name</td>
                      <td>Surcharge</td>
                      <td>SKU</td>
                      <td>
                        <el-image
                          style="width: 60px; height: 60px"
                          src="https://fuss10.elemecdn.com/a/3f/3302e58f9a181d2509f3dc0fa68b0jpeg.jpeg"
                          fit="cover"
                          :preview-src-list="['https://fuss10.elemecdn.com/a/3f/3302e58f9a181d2509f3dc0fa68b0jpeg.jpeg']"
                        />
                      </td>
                    </tr>
                    <tr>
                      <td>
                        <input type="checkbox" name="checked" :value="2" v-model="checkBoxValue" />
                      </td>
                      <td>Name</td>
                      <td>Surcharge</td>
                      <td>SKU</td>
                      <td>
                        <el-image
                          style="width: 60px; height: 60px"
                          src="https://fuss10.elemecdn.com/a/3f/3302e58f9a181d2509f3dc0fa68b0jpeg.jpeg"
                          fit="cover"
                          :preview-src-list="['https://fuss10.elemecdn.com/a/3f/3302e58f9a181d2509f3dc0fa68b0jpeg.jpeg']"
                        />
                      </td>
                    </tr>
                  </table>
                  <h5>{{ ob.polyT('listingDetail.tags') }}</h5>
                  <div class="tagWrapper rowLg">
                    <template v-for="tag in ob.item.tags">
                      <a class="btn tag clrSh2 clrBr" :href="`#search?q=${tag}`" v-html="`#${ob.parseEmojis(tag)}`"></a>
                    </template>
                    <template v-if="!ob.item.tags.length">
                      <i class="clrT2">{{ ob.polyT('listingDetail.noTags') }}</i>
                    </template>
                  </div>
                  <h5>{{ ob.polyT('listingDetail.paymentsAccepted') }}</h5>
                  <div class="js-supportedCurrenciesList">
                    <SupportedCurrenciesList
                      :options="{
                        initialState: {
                          currencies: model.get('metadata').get('acceptedCurrencies'),
                        },
                      }"
                    />
                  </div>
                  <template v-if="ob.hasVerifiedMods">
                    <div class="verifiedModBox clrBrAlert2 clrBAlert2Grad">
                      <div class="flexVCent flexHCent gutterHTn rowSm">
                        <div class="badge" :style="`background-image: url(${ob.defaultBadge.tiny}), url('../imgs/verifiedModeratorBadgeDefault.png');`"></div>
                        <div class="tx5 txB">{{ ob.polyT('verifiedMod.modVerified.titleLong') }}</div>
                      </div>
                      <div class="flexColRows gutterVSm tx5b">
                        <div
                          v-html="
                            ob.polyT('verifiedMod.genericDescription', {
                              name: `<b>${ob.verifiedModsData.name}</b>`,
                              link: `<a class=&quot;txU noWrap&quot; href=&quot;${ob.verifiedModsData.link}&quot; data-open-external>${ob.polyT(
                                'verifiedMod.link'
                              )}</a>`,
                            })
                          "
                        ></div>
                      </div>
                    </div>
                  </template>
                </div>
              </div>
            </div>

            <div class="contentBox descriptionSection padLg clrP clrBr clrSh3">
              <h2 class="txUnb">{{ ob.polyT('listingDetail.description') }}</h2>
              <div v-html="ob.item.description" />
              <template v-if="!ob.item.description">
                <i class="clrT2">{{ ob.polyT('listingDetail.noDescription') }}</i>
              </template>
            </div>

            <template v-if="ob.item.images.length">
              <div ref="photoSection" class="contentBox clrSh3 photoSection js-photoSection">
                <div ref="photoSelected" class="flexCent photoSelected js-photoSelected">
                  <img ref="photoSelectedInner" class="photoSelectedInner js-photoSelectedInner" />
                </div>
                <template v-if="ob.item.images.length > 1">
                  <button class="btn ion-ios-arrow-left photoPrev" @click="onClickPhotoPrev"></button>
                  <button class="btn ion-ios-arrow-right photoNext" @click="onClickPhotoNext"></button>
                </template>
                <template v-if="ob.item.images.length > 1">
                  <div class="photoStrip flex gutterH">
                    <template v-for="(image, photoIndex) in ob.item.images">
                      <input
                        type="radio"
                        name="photoStripThumbnails"
                        class="js-photoSelect"
                        :id="`photoStrip${photoIndex}`"
                        :value="photoIndex"
                        v-model="activePhotoIndex"
                        @click="onClickPhotoSelect(photoIndex)"
                      />
                      <label
                        :style="`background-image: url(` + ob.getServerUrl(`ob/image/${ob.isHiRez() ? image.small : image.tiny}`) + `)`"
                        :for="`photoStrip${photoIndex}`"
                      ></label>
                    </template>
                  </div>
                </template>
              </div>
            </template>
            <div class="reviews js-reviews">
              <Reviews
                ref="reviews"
                :key="reviewIDs"
                :reviewIDs="reviewIDs"
                :options="{
                  async: true,
                  showListingData: true,
                  isFetchingRatings: !ratingData.fetched,
                }"
              />
            </div>

            <!-- Attachments are not yet available -->
            <!--

    <div class="contentBox padLg clrP clrBr clrSh3">
      <h2 class="txUnb">{{ ob.polyT('listingDetail.attachments') }}</h2>
      Placeholder for Attachments
    </div>
  -->

            <template v-if="ob.shippingOptions.length">
              <div ref="shippingSection" class="contentBox padLg clrP clrBr clrSh3" id="shippingSection">
                <h2 class="txUnb">{{ ob.polyT('listingDetail.shipping') }}</h2>
                <div class="flexVCent gutterHLg tx5">
                  <!-- this data is not yet available -->
                  <!--
          <div>{{ ob.polyT('listingDetail.shipsFrom', { country: `<b>insert translation of the country here</b>` }) }}</div>
          -->
                  <div>{{ ob.polyT('listingDetail.shipTo') }}</div>
                  <div class="col4">
                    <Select2 v-model="shippingDestination">
                      <option value="ALL">{{ ob.polyT('listingDetail.allCountries') }}</option>
                      <template v-for="country in countryData">
                        <option :value="country.id" :selected="country.id === shippingDestination">{{ country.text }}</option>
                      </template>
                    </Select2>
                  </div>
                </div>
                <div class="js-shippingOptions">
                  <ShippingOptions :options="shippingOptionsInfo" :key="shippingDestination" />
                </div>
              </div>
            </template>
            <div class="contentBox padLg clrP clrBr clrSh3">
              <h2 class="txUnb">{{ ob.polyT('listingDetail.refundPolicy') }}</h2>
              <div v-html="ob.refundPolicy" />
              <template v-if="!ob.refundPolicy">
                <i class="clrT2">{{ ob.polyT('listingDetail.noRefundPolicy') }}</i>
              </template>
            </div>

            <div class="contentBox padLg clrP clrBr clrSh3">
              <h2 class="txUnb">{{ ob.polyT('listingDetail.termsAndConditions') }}</h2>
              <div v-html="ob.termsAndConditions" />
              <template v-if="!ob.termsAndConditions">
                <i class="clrT2">{{ ob.polyT('listingDetail.noTermsAndConditions') }}</i>
              </template>
            </div>

            <div class="js-moreListings">
              <MoreListings
                :options="{
                  vendor,
                  listings: moreListingsData,
                }"
              />
            </div>
          </div>
        </template>
      </BaseModal>
    </div>
    <Teleport to="#js-vueModal">
      <NsfwWarning v-if="showNsfwWarning" @canceled="close" @close="onNsfwWarningClose" />
      <Purchase
        ref="purchaseModal"
        v-else-if="showPurchase"
        :options="{ itemsInfo: [{ quantity: '1', variants: selectedVariants }], vendor, origin: 'Listing' }"
        :bb="
          function () {
            return {
              itemsToPurchase,
            };
          }
        "
        @clickReloadOutdated="onPurchaseReloadOutdated"
        @close="onPurchaseClose"
      />
      <EditListing
        ref="editModal"
        v-else-if="showEditListing"
        :options="{
          returnText: ob.polyT('listingDetail.editListingReturnText'),
          onClickViewListing: onEditModalClickReturn,
        }"
        :bb="
          function () {
            return {
              model,
            };
          }
        "
        @click-return="onEditModalClickReturn"
        @close="onCloseEditModal"
      />
      <EditListing
        ref="cloneModal"
        v-else-if="showCloneListing"
        :bb="
          function () {
            return {
              model: model.cloneListing(),
            };
          }
        "
        @close="onCloseCloneModal"
      />
    </Teleport>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import { Collection } from 'backbone';
import bigNumber from 'bignumber.js';
import 'jquery-zoom';
import is from 'is_js';
import app from '../../../../backbone/app';
import 'velocity-animate';
import { convertAndFormatCurrency } from '../../../../backbone/utils/currency';
// import {
//   getInventory,
//   events as inventoryEvents,
// } from '../../../utils/inventory';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { events as outdatedListingHashesEvents } from '../../../../backbone/utils/outdatedListingHashes';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
// import QuantityDisplay from '../../components/QuantityDisplay';
import { events as listingEvents } from '../../../../backbone/models/listing';
import Listings from '../../../../backbone/collections/Listings';

import OrderListings from '../../../../backbone/collections/OrderListings';

import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';

import PopInMessage, { buildRefreshAlertMessage } from '../../../../backbone/views/components/PopInMessage';

import api from '../../../api';

import Rating from './Rating.vue';
import NsfwWarning from '../NsfwWarning.vue';
import MoreListings from './MoreListings.vue';
import ShippingOptions from './ShippingOptions.vue';
import Reviews from '../../reviews/Reviews.vue';
import PurchaseError from '@/views/modals/listingDetail/PurchaseError.vue';
import Purchase from '../purchase/Purchase.vue';
import EditListing from '../editListing/EditListing.vue';

export default {
  components: {
    Rating,
    NsfwWarning,
    MoreListings,
    ShippingOptions,
    Reviews,
    Purchase,
    PurchaseError,
    EditListing,
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
      isCheckAll: false,
      checkBoxValue: [],
      app,

      showModal: true,

      PURCHASE_MODAL_CREATE: 'PURCHASE_MODAL_CREATE',

      outdateHash: false,

      vendor: undefined,

      variantOptions: [],

      activePhotoIndex: 0,

      shippingDestination: this.defaultCountry(),
      countryData: getTranslatedCountries().map((countryObj) => ({ id: countryObj.dataName, text: countryObj.name })),

      ratingData: {
        averageRating: 0,
        ratingCount: 0,
        fetched: false,
      },
      reviewIDs: [],

      _showNsfwWarning: true,

      moreListingsData: undefined,

      isDeleting: false,
      showDeleteConfirmedBox: false,

      showPurchase: false,
      showEditListing: false,
      showCloneListing: false,
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
    '_model.item.options'(options) {
      this.variantOptions = [];
      for (let option of options) {
        if (option.variants && option.variants.length > 0) {
          this.variantOptions.push(option.variants[0]);
        } else {
          this.variantOptions.push('');
        }
      }
    },
  },
  unmounted() {
    if (this.destroyRequest) this.destroyRequest.abort();
    if (this.ratingsFetch) this.ratingsFetch.abort();
    // if (this.inventoryFetch) this.inventoryFetch.abort();
    if (this.moreListingsFetch) this.moreListingsFetch.abort();
  },
  computed: {
    ob() {
      const defaultBadge = app.verifiedMods.defaultBadge(this.model.get('moderators'));
      const flatModel = this.model.toJSON();

      return {
        ...this.templateHelpers,
        ...flatModel,
        shipsFreeToMe: this.shipsFreeToMe,
        ownListing: this.model.isOwnListing,
        price: this.model.price,
        displayCurrency: app.settings.get('localCurrency'),
        // the ships from data doesn't exist yet
        // shipsFromCountry: this.model.get('shipsFrom');
        openedFromStore: this.options.openedFromStore,
        hasVerifiedMods: this.hasVerifiedMods,
        verifiedModsData: app.verifiedMods.data,
        defaultBadge,
        isCrypto: this.model.isCrypto,
        _: { sortBy: _.sortBy },
      };
    },
    itemsToPurchase() {
      return new OrderListings([this.model]);
    },
    templateOptions() {
      const ob = this.ob;

      let tip;
      let buyNowClass = 'disabled';
      let buyNowTranslationKey = ob.metadata.contractType !== 'CRYPTOCURRENCY' ? 'listingDetail.buyNow' : 'listingDetail.buyCryptoNow';
      let unpurchaseable = true;

      let coinTypeRateAvailable;
      let cryptoPaymentCoinRateAvailable;

      if (ob.metadata.contractType === 'CRYPTOCURRENCY') {
        coinTypeRateAvailable = !!ob.currencyMod.getExchangeRate(ob.item.cryptoListingCurrencyCode);
        cryptoPaymentCoinRateAvailable = !!ob.currencyMod.getExchangeRate(ob.metadata.acceptedCurrencies[0]);
      }

      if (!ob.crypto.anySupportedByWallet(ob.metadata.acceptedCurrencies)) {
        tip = ob.polyT('listingDetail.unableToPurchase.incompatibleCrypto', {
          acceptedCurs: ob.metadata.acceptedCurrencies.join(', '),
          walletCurs: ob.crypto.supportedWalletCurs().join(', '),
        });
      } else if (
        ob.metadata.contractType !== 'CRYPTOCURRENCY' &&
        !ob.currencyMod.getExchangeRate(ob.price.currencyCode) &&
        !(ob.crypto.supportedWalletCurs().includes(ob.price.currencyCode) && ob.metadata.acceptedCurrencies.includes(ob.price.currencyCode))
      ) {
        // If it's priced in a wallet cur and that cur is one of the accepted
        // curs, we won't disable purchase even if there's no exchange rate for the
        // cur because they could still pay for it using that cur making the
        // pricing and payment curs the same and therefore the exchange rate
        // unnecessary.
        tip = ob.polyT('listingDetail.unableToPurchase.noExchangeRateInfo', {
          cur: ob.price.currencyCode,
        });
      } else if (
        ob.metadata.contractType === 'CRYPTOCURRENCY' &&
        ob.item.cryptoListingCurrencyCode !== ob.metadata.acceptedCurrencies[0] &&
        (!coinTypeRateAvailable || !cryptoPaymentCoinRateAvailable)
      ) {
        const cursNoRate = [];
        if (!coinTypeRateAvailable) cursNoRate.push(ob.item.cryptoListingCurrencyCode);
        if (!cryptoPaymentCoinRateAvailable) cursNoRate.push(ob.metadata.acceptedCurrencies[0]);
        tip = ob.polyT('listingDetail.unableToPurchase.noCryptoExchangeRateInfo', {
          cur: cursNoRate.join(', '),
        });
      } else {
        buyNowClass = '';
        unpurchaseable = false;
      }

      return { tip, buyNowClass, buyNowTranslationKey, unpurchaseable };
    },

    shippingOptionsInfo() {
      const shippingOptions = this.model.get('shippingOptions');
      const filteredOptions = shippingOptions.filter((option) => {
        if (this.shippingDestination === 'ALL') return option.get('regions');
        return option.get('regions').includes(this.shippingDestination);
      });

      return {
        shippingOptions: filteredOptions,
        displayCurrency: app.settings.get('localCurrency'),
      };
    },
    showNsfwWarning() {
      return this._showNsfwWarning && this.checkNsfw && this.model.get('item').get('nsfw') && !this.model.isOwnListing && !app.settings.get('showNsfw');
    },
    totalPriceInfo() {
      let priceInfo;
      try {
        priceInfo = convertAndFormatCurrency(this.totalPrice, this.model.get('metadata').get('pricingCurrency').code, app.settings.get('localCurrency'));
      } catch (e) {
        // pass
        console.error(e);
      }
      return priceInfo;
    },
    selectedVariants() {
      const { options } = this.model.toJSON().item;

      return this.variantOptions.map((val, idx) => ({
        name: options[idx].name,
        value: val,
      }));
    },
    cryptoTradingPairOptions() {
      if (this.model.isCrypto) {
        const metadata = this.model.get('metadata');

        // if (this.cryptoInventory) this.cryptoInventory.remove();
        // this.cryptoInventory = this.createChild(QuantityDisplay, {
        //   peerID: this.vendor.peerID,
        //   slug: this.model.get('slug'),
        //   initialState: {
        //     coinType: metadata.get('coinType'),
        //     amount: this._inventory,
        //   },
        // });
        // $('.js-cryptoInventory')
        //   .html(this.cryptoInventory.render().el);

        return {
          tradingPairClass: 'cryptoTradingPairXL rowSm',
          exchangeRateClass: 'clrT2 exchangeRateLine',
          fromCur: metadata.get('acceptedCurrencies')[0],
          toCur: this.model.get('item').get('cryptoListingCurrencyCode'),
        };
      }
      return {};
    },
  },
  watch: {
    checkBoxValue: {
      handler(values) {
        let data = [1, 2];
        this.isCheckAll = data.every((item) => values.includes(item));
      },
      immediate: true,
    },
  },
  methods: {
    changeCheckAll(e) {
      let data = [1, 2];
      this.checkBoxValue = e.target.checked ? [...data] : [];
    },
    loadData(options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      const opts = {
        checkNsfw: true,
        removeOnClose: true,
        ...options,
      };

      this.baseInit(opts);

      this.shipsFreeToMe = this.model.shipsFreeToMe;
      this.activePhotoIndex = 0;

      // Set to an empty bigNumber instance so if we can't fill it with a legitmate
      // value, at least bigNumber ops won't fail.
      this.totalPrice = bigNumber();

      try {
        this.totalPrice = this.model.get('item').get('price');
      } catch (e) {
        // pass
      }

      this._latestHash = this.model.get('hash');
      this._renderedHash = null;

      // Sometimes a profile model is available and the vendor info
      // can be obtained from that.
      const profile = this.profile;
      if (profile) {
        this.vendor = {
          peerID: profile.id,
          name: profile.get('name'),
          handle: profile.get('handle'),
          avatarHashes: profile.get('avatarHashes').toJSON(),
        };
      }

      // In most cases the page opening this modal will already have and be able
      // to provide the vendor information. If it cannot, then I suppose we
      // could fetch the profile and lazy load it in, but we can cross that
      // bridge when we get to it.
      this.vendor = this.vendor || opts.vendor;

      this.variantOptions = [];
      const itemOptions = this.model.get('item').get('options').toJSON();
      for (let option of itemOptions) {
        if (option.variants && option.variants.length > 0) {
          this.variantOptions.push(option.variants[0].name);
        } else {
          this.variantOptions.push('');
        }
      }

      this.listenTo(app.settings, 'change:country', () => (this.shipsFreeToMe = this.model.shipsFreeToMe));

      this.listenTo(app.settings.get('shippingAddresses'), 'update', (cl, updateOpts) => {
        if (updateOpts.changes.added.length || updateOpts.changes.removed.length) {
          this.shipsFreeToMe = this.model.shipsFreeToMe;
        }
      });

      if (this.model.isOwnListing) {
        this.listenTo(listingEvents, 'saved', (md, e) => {
          const slug = this.model.get('slug');
          if (e.slug === slug) {
            // Factoring out the inventory from the listing data because
            // the inventory will auto-update on a change - no need for a
            // refresh pop-up if that's the only thing that changed.
            const { prev } = e;
            delete prev.item.cryptoQuantity;

            const cur = md.toJSON();
            delete cur.item.cryptoQuantity;

            if (!_.isEqual(prev, cur)) {
              this.showDataChangedMessage();
            }
          }
        });

        this.listenTo(app.settings, 'change:localCurrency', () => this.showDataChangedMessage());
        this.listenTo(app.localSettings, 'change:bitcoinUnit', () => this.showDataChangedMessage());
      }

      this.hasVerifiedMods = app.verifiedMods.matched(this.model.get('moderators')).length > 0;

      this.listenTo(app.verifiedMods, 'update', () => {
        const newHasVerifiedMods = app.verifiedMods.matched(this.model.get('moderators')).length > 0;
        if (newHasVerifiedMods !== this.hasVerifiedMods) {
          this.hasVerifiedMods = newHasVerifiedMods;
          this.showDataChangedMessage();
        }
      });

      this.listenTo(outdatedListingHashesEvents, 'newHash', (e) => {
        this._latestHash = e.newHash;
        if (e.oldHash === this._renderedHash) this.outdateHash = true;
      });

      // get the ratings data, if any
      this.ratingsFetch = $.get(app.getServerUrl(`ob/ratingindex/${this.vendor.peerID}/${this.model.get('slug')}`))
        .done((data) => this.onRatings(data))
        .fail((jqXhr) => {
          if (jqXhr.statusText === 'abort') return;
          const failReason = (jqXhr.responseJSON && jqXhr.responseJSON.reason) || '';
          openSimpleMessage(app.polyglot.t('listingDetail.errors.fetchRatings'), failReason);
        });

      if (this.model.isCrypto) {
        // Commenting out for since inventory fetch is currently broken on the server.
        // startAjaxEvent('Listing_InventoryFetch');
        // this.inventoryFetch = getInventory(this.vendor.peerID, {
        //   slug: this.model.get('slug'),
        //   coinDivisibility: this.model.get('metadata')
        //     .get('coinDivisibility'),
        // })
        //   .done(e => {
        //     this._inventory = e.inventory;
        //     if (this.cryptoInventory) {
        //       this.cryptoInventory.setState({
        //         amount: this._inventory,
        //       });
        //     }
        //     endAjaxEvent('Listing_InventoryFetch', {
        //       ownListing: this.model.isOwnListing,
        //     });
        //   })
        //   .fail(e => {
        //     endAjaxEvent('Listing_InventoryFetch', {
        //       ownListing: this.model.isOwnListing,
        //       errors: e.error || e.errCode || 'unknown error',
        //     });
        //   });
        // this.listenTo(inventoryEvents, 'inventory-change',
        //   e => (this._inventory = e.inventory));
      }

      this.moreListingsCol = new Listings([], { guid: this.vendor.peerID });

      const fetchOpts =
        this.vendor.peerID === app.profile.id
          ? {}
          : {
              data: $.param({
                'max-age': 60 * 60, // 1 hour
              }),
            };

      this.moreListingsFetch = this.moreListingsCol.fetch(fetchOpts).done(() => {
        this.moreListingsData = this.randomizeMoreListings(this.moreListingsCol);
      });

      this._outdatedHashState = null;
    },

    onDocumentClick() {
      this.showDeleteConfirmedBox = false;
    },

    defaultCountry() {
      return app.settings.get('shippingAddresses').length ? app.settings.get('shippingAddresses').at(0).get('country') : app.settings.get('country');
    },

    onRatings(data) {
      const pData = data || {};

      this.ratingData = {
        averageRating: pData.average,
        ratingCount: pData.count,
        fetched: true,
      };

      this.reviewIDs = pData.ratings || [];
    },

    onClickEditListing() {
      recordEvent('Listing_EditFromListing');

      this.showEditListing = true;
      this.showModal = false;
    },

    onEditModalClickReturn() {
      this.$refs.editModal.confirmClose().done(() => {
        this.showEditListing = false;

        this.showModal = true;
      });
    },

    onCloseEditModal() {
      this.showEditListing = false;
      this.showModal = true;
    },

    onClickCloneListing() {
      recordEvent('Listing_CloneFromListing');

      this.showCloneListing = true;
      this.showModal = false;
    },

    onCloseCloneModal() {
      this.showCloneListing = false;
      this.showModal = true;
    },

    onClickDeleteListing() {
      recordEvent('Listing_DeleteFromListing');
      this.showDeleteConfirmedBox = true;
    },

    onClickConfirmedDelete() {
      recordEvent('Listing_DeleteFromListingConfirm');
      if (this.destroyRequest && this.destroyRequest.state === 'pending') return;
      this.destroyRequest = this.model.destroy({ wait: true });

      if (this.destroyRequest) {
        this.isDeleting = true;

        this.destroyRequest
          .done(() => {
            if (this.destroyRequest.statusText === 'abort' || this.isRemoved()) return;

            this.close();
          })
          .always(() => {
            this.isDeleting = false;
          });
      }
    },

    onClickConfirmCancel() {
      recordEvent('Listing_DeleteFromListingCancel');
      this.showDeleteConfirmedBox = false;
    },

    onClickGotoPhotos() {
      recordEvent('Listing_GoToPhotos', { ownListing: this.model.isOwnListing });
      this.gotoPhotos();
    },

    onClickGoToStore() {
      if (this.options.openedFromStore) {
        recordEvent('Listing_GoToStore', {
          OpenedFromStore: true,
          ownListing: this.model.isOwnListing,
        });
        this.close();
      } else {
        recordEvent('Listing_GoToStore', {
          OpenedFromStore: false,
          ownListing: this.model.isOwnListing,
        });
        const base = this.vendor.handle ? `@${this.vendor.handle}` : this.vendor.peerID;
        app.router.navigateUser(`${base}/store`, this.vendor.peerID, { trigger: true });
      }
    },

    randomizeMoreListings(cl) {
      if (!(cl instanceof Collection)) {
        throw new Error('Please provide a Collection instance.');
      }

      return _.shuffle(cl.models)
        .filter((md) => md.get('slug') !== this.model.get('slug'))
        .map((md) => md.toJSON())
        .slice(0, 8);
    },

    gotoPhotos() {
      recordEvent('Listing_GoToPhotos', { ownListing: this.model.isOwnListing });

      this.scrollToSection('.photoSection');
    },

    clickRating() {
      recordEvent('Listing_ClickOnRatings', { ownListing: this.model.isOwnListing });
      this.scrollToSection('.reviews');
    },

    scrollToSection(el) {
      this.$scrollTo(el, 500, {
        offset: -10,
        container: '.listingDetail', //设置滚动容器
        easing: 'ease-out', //动画效果
        x: false, //是否在x轴滚动
        y: true, //是否在y轴滚动
      });
    },

    onClickPhotoSelect(photoIndex) {
      recordEvent('Listing_ClickOnPhoto', { ownListing: this.model.isOwnListing });
      this.setSelectedPhoto(photoIndex);
    },

    setSelectedPhoto(photoIndex) {
      if (is.not.number(photoIndex)) {
        throw new Error('Please provide an index for the selected photo.');
      }
      if (photoIndex < 0) {
        throw new Error('Please provide a valid index for the selected photo.');
      }
      const photoCol = this.model.toJSON().item.images;
      const photoHash = photoCol[photoIndex].original;
      const phSrc = app.getServerUrl(`ob/image/${photoHash}`);

      this.activePhotoIndex = photoIndex;
      this.getPhotoSelectedEl().trigger('zoom.destroy'); // old zoom must be removed
      this.photoSelectedInner.attr('src', phSrc);
    },

    activateZoom() {
      if (this.photoSelectedInner.width() >= this.getPhotoSelectedEl().width() || this.photoSelectedInner.height() >= this.getPhotoSelectedEl().height()) {
        this.getPhotoSelectedEl()
          .removeClass('unzoomable')
          .zoom({
            url: this.photoSelectedInner.attr('src'),
            on: 'click',
            onZoomIn: () => {
              this.getPhotoSelectedEl().addClass('open');
            },
            onZoomOut: () => {
              this.getPhotoSelectedEl().removeClass('open');
            },
          });
      } else {
        this.getPhotoSelectedEl().addClass('unzoomable');
      }
    },

    onClickPhotoPrev() {
      recordEvent('Listing_ClickOnPhotoPrev', { ownListing: this.model.isOwnListing });
      let targetIndex = this.activePhotoIndex - 1;
      const imagesLength = parseInt(this.model.toJSON().item.images.length, 10);

      targetIndex = targetIndex < 0 ? imagesLength - 1 : targetIndex;
      this.setSelectedPhoto(targetIndex);
    },

    onClickPhotoNext() {
      recordEvent('Listing_ClickOnPhotoNext', { ownListing: this.model.isOwnListing });
      let targetIndex = this.activePhotoIndex + 1;
      const imagesLength = parseInt(this.model.toJSON().item.images.length, 10);

      targetIndex = targetIndex >= imagesLength ? 0 : targetIndex;
      this.setSelectedPhoto(targetIndex);
    },

    onClickFreeShippingLabel() {
      recordEvent('Listing_ClickFreeShippingLabel', { ownListing: this.model.isOwnListing });
      this.gotoShippingOptions();
    },

    gotoShippingOptions() {
      $(this.$refs.shippingSection).velocity('scroll', {
        duration: 500,
        easing: 'easeOutSine',
        container: this.$el,
      });
    },

    onChangeVariantSelect() {
      this.adjustPriceBySku();
    },

    adjustPriceBySku() {
      const { options } = this.model.toJSON().item;
      const selections = this.variantOptions.map((val, idx) => ({
        option: options[idx].name,
        variant: val,
      }));

      // each sku has a code that matches the selected variant index combos
      const sku = this.model
        .get('item')
        .get('skus')
        .find((v) => _.isEqual(v.get('selections'), selections));
      const surcharge = sku ? sku.get('surcharge') : bigNumber('0');

      try {
        const _totalPrice = this.model.price.amount.plus(surcharge || bigNumber('0'));
        if (!_totalPrice.eq(this.totalPrice)) {
          this.totalPrice = _totalPrice;
        }
      } catch (e) {
        // pass
      }
    },

    showDataChangedMessage() {
      if (this.dataChangePopIn && !this.dataChangePopIn.isRemoved()) {
        this.dataChangePopIn.$el.velocity('callout.shake', { duration: 500 });
      } else {
        this.dataChangePopIn = this.createChild(PopInMessage, {
          messageText: buildRefreshAlertMessage(app.polyglot.t('listingDetail.listingDataChangedPopin')),
        });

        this.listenTo(this.dataChangePopIn, 'clickRefresh', () => this.$emit('refresh'));

        this.listenTo(this.dataChangePopIn, 'clickDismiss', () => {
          this.dataChangePopIn.remove();
          this.dataChangePopIn = null;
        });

        $(this.$refs.popInMessages).append(this.dataChangePopIn.render().el);
      }
    },

    onClickReloadOutdated(e) {
      if (e.target.id !== 'reloadOutdated') return;

      this.$emit('refresh');
    },

    startPurchase() {
      if (!this.model.isCrypto) {
        if (this.totalPrice.lte(0)) {
          openSimpleMessage(app.polyglot.t('listingDetail.errors.noPurchaseTitle'), app.polyglot.t('listingDetail.errors.zeroPriceMsg'));
          return;
        }
        // Commenting out inventory related stuff for now since it's broken on the server.
        // } else {
        //   if (
        //     typeof this._inventory === 'number' &&
        //     this._inventory <= 0
        //   ) {
        //     openSimpleMessage(app.polyglot.t('listingDetail.errors.noPurchaseTitle'),
        //       app.polyglot.t('listingDetail.errors.outOfStock'));
        //     return;
        //   }
      }

      this.showPurchase = true;

      recordEvent('Purchase_Start', { ownListing: this.model.isOwnListing });
    },

    onPurchaseReloadOutdated() {
      this.showPurchase = false;
      this.showModal = true;
    },
    onPurchaseClose() {
      this.showPurchase = false;
      this.close();
    },

    addToCart() {
      api.addToShoppingCart(this.vendor.peerID, {
        slug: this.model.get('slug'),
        quantity: '1',
        options: this.selectedVariants || [],
      });
    },

    getPhotoSelectedEl() {
      return $(this.$refs.photoSelected);
    },

    onNsfwWarningClose() {
      this._showNsfwWarning = false;
    },

    render() {
      if (this.dataChangePopIn) this.dataChangePopIn.remove();

      if (this._latestHash !== this.model.get('hash')) {
        this.outdateHash = true;
      }

      this.photoSelectedInner = $(this.$refs.photoSelectedInner);

      this.photoSelectedInner.on('load', () => this.activateZoom());

      this.setSelectedPhoto(this.activePhotoIndex);

      if (!this.model.isCrypto) {
        this.adjustPriceBySku();
      }

      this._renderedHash = this.model.get('hash');

      return this;
    },
  },
};
</script>
<style lang="scss" scoped>
.flex {
  &-1 {
    flex: 1;
  }
  button {
    font-size: 16px;
    padding: 0 12px;
  }
  button + button {
    margin-left: 10px;
  }
}
.warning-btn,
.success-btn {
  height: 52px;
  border-radius: 26px;
  background-color: #eb9b3a;
  border-width: 1px;
  border-style: solid;
  border-color: #e8b16e;
  font-size: 18px;
  color: #fff;
  box-sizing: border-box;
  &:hover {
    opacity: 0.8;
  }
}
.success-btn {
  background-color: #01bf65;
  border-color: #66e9ac;
}
.table {
  border-spacing: 0;
  border-collapse: collapse;
  width: 100%;
  margin-bottom: 20px;
  input[type='checkbox'] {
    display: inline-block;
  }
  th,
  td {
    border-width: 1px;
    border-style: solid;
    padding: 10px;
    word-break: break-word;
    box-sizing: border-box;
  }
}
</style>
