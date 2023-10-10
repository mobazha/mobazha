<template>
  <div class="modal listingDetail modalScrollPage" v-show="showModal" @click="onDocumentClick">
    <BaseModal>
      <template v-slot:component>
        <div class="popInMessageHolder js-popInMessages"></div>

        <div class="topControls withEndBtn flex">
          <template v-if="ob.vendor">
            <div class="contentBox clrP clrSh3 clrBr clrT">
              <div class="padSm gutterHSm overflowAuto margRSm flexVCent">
                <a class="clrBr2 clrSh1 disc storeOwnerAvatar flexNoShrink js-storeOwnerAvatar" :style="ob.getAvatarBgImage(ob.vendor.avatarHashes)"></a>
                <p class="txUnl tx3 clamp">{{ ob.vendor.name }}</p>
                <a class="link flexNoShrink tx6 " @click="onClickGoToStore">{{ ob.openedFromStore ? ob.polyT('listingDetail.returnToStore') : ob.polyT('listingDetail.goToStore') }}</a>
              </div>
            </div>
          </template>
          <template v-if="ob.ownListing">
            <div class="flexNoShrink" style="margin-left: auto">
              <div class="btnStrip clrSh3">
                <button class="btn clrP clrBr" @click="onClickEditListing">{{ ob.polyT('listingDetail.edit') }}</button>
                <button class="btn clrP clrBr" @click="onClickCloneListing">{{ ob.polyT('listingDetail.clone') }}</button>
                <ProcessingButton
                  className="btn js-deleteListing clrP clrBr"
                  @click="onClickDeleteListing"
                  :btnText="ob.polyT('listingDetail.delete')" />
              </div>
            </div>
            <div class=" confirmBox deleteConfirm tx5 arrowBoxTop clrBr clrP clrT hide" @click="onClickDeleteConfirmBox">
              <div class="tx3 txB rowSm">{{ ob.polyT('listingDetail.confirmDelete.title') }}</div>
              <p>{{ ob.polyT('listingDetail.confirmDelete.body') }}</p>
              <hr class="clrBr row" />
              <div class="flexHRight flexVCent gutterHLg buttonBar">
                <a class="" @click="onClickConfirmCancel">{{ ob.polyT('listingDetail.confirmDelete.btnCancel') }}</a>
                <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph " @click="onClickConfirmedDelete">{{ ob.polyT('listingDetail.confirmDelete.btnConfirm') }}</a>
              </div>
            </div>
          </template>

          <template v-else>
            <div class="flexNoShrink" style="margin-left: auto">
              <div class="js-socialBtns"></div>
            </div>
          </template>
        </div>


        <div class="listingContent flexColRow gutterVMd2">
          <div class="contentBox padLg clrP clrBr clrSh3">
            <div :class="`${ob.metadata.contractType !== 'CRYPTOCURRENCY' ? 'flex' : 'flexVCent'} gutterHLg`">
              <template v-if="ob.metadata.contractType !== 'CRYPTOCURRENCY'">
                <h2 class="txUnb flexExpand">{{ ob.item.title }}</h2>
                <h2 class="txUnb flexNoShrink js-price">
                  {{
                    ob.currencyMod.convertAndFormatCurrency(
                      ob.price.amount,
                      ob.price.currencyCode,
                      ob.displayCurrency
                    )
                  }}
                </h2>
              </template>

              <template v-else>
                <h2 class="flexExpand js-cryptoTitle cryptoTitle"></h2>
                <CryptoPrice :options="{
                      priceAmount: ob.price.amount,
                      priceCurrencyCode: ob.price.currencyCode,
                      displayCurrency: ob.displayCurrency,
                      priceModifier: ob.price.modifier,
                      wrappingTag: 'h2',
                      wrappingClass: 'flexNoShrink txRgt tx3',
                    }" />
              </template>
            </div>
            <div class="flex gutterHLg">
              <div class="mainImageWrapper">
                <div class="mainImage clrBr " @click="onClickGotoPhotos" :style="ob.item.images.length
                  ? `background-image: url(${ob.getServerUrl(`ob/image/${ob.isHiRez() ? ob.item.images[0].large : ob.item.images[0].medium}`)}), url('../imgs/defaultItem.png')`
                  : `background-image: url('../imgs/defaultItem.png')`"></div>
                <div class="txCtr">
                  <a class="tx5 " @click="onClickGotoPhotos">
                    <u>{{ ob.polyT('listingDetail.viewPhotos', {
                      count: ob.item.images.length, smart_count:
                        ob.item.images.length
                    }) }}</u>
                  </a>
                </div>
              </div>
              <div class="flexExpand">
                <div class="buyBox clrP clrBr">
                  <div class="flexColRows flexHCent gutterV">
                    <template v-for="(item, j) in ob.item.options" :key="j">
                      <div class="flexVCent gutterHLg">
                        <div class="col4 h5 txUnl">{{ item.name }}</div>
                        <div class="col8 txLft">
                          <select class="" @change="onChangeVariantSelect" :name="item.name">
                            <template v-for="(variant, j) in item.variants" :key="j">
                              <option :value="variant.name">{{ variant.name }}</option>
                            </template>
                          </select>
                        </div>
                      </div>
                    </template>

                    <button
                      :class="`btnHg clrBAttGrad clrBrDec1 clrTOnEmph js-purchaseBtn ${templateOptions.buyNowClass} ${outdateHash ? 'disabled' : ''}`"
                      @click="startPurchase">
                      {{ ob.polyT(templateOptions.buyNowTranslationKey) }}
                    </button>
                    <button
                      :class="`btnHg clrBAttGrad clrBrDec1 clrTOnEmph js-addToCartBtn ${templateOptions.buyNowClass}`"
                      @click="addToCart">
                      {{ ob.polyT('listingDetail.addToCart') }}
                    </button>

                    <div class="js-purchaseErrorWrap">
                      <template v-if="outdateHash">
                        <PurchaseError :tip='ob.polyT("listingDetail.errors.outdatedHash", {
                          reloadLink: `<a class="js-reloadOutdated">` + `${ob.polyT("listingDetail.errors.reloadOutdatedHash")}<a>`,
                        })'></PurchaseError>
                      </template>
                      <template v-else-if="unpurchaseable">
                        <PurchaseError :tip="templateOptions.tip"></PurchaseError>
                      </template>
                    </div>

                    <div class="flexHCent gutterH">
                      <div class="tx6  rating" @click="clickRating"></div>
                      <template v-if="ob.shipsFreeToMe">
                        <div class="txCtr">
                          <a class="clrE1 clrTOnEmph phraseBox txNoUnd " @click="onClickFreeShippingLabel">{{
                            ob.polyT('listingDetail.freeShippingBanner') }}</a>
                        </div>
                      </template>
                    </div>
                  </div>
                </div>
                <div class="flexHCent gutterHLg tx5 rowLg">
                  <div>
                    {{ ob.polyT('listingDetail.type', {
                      type: `<b>${ob.polyT(`formats.${ob.metadata.contractType}`)}</b>`
                    }) }}
                  </div>
                  <!-- // not showing the inventory for now since it's broken on the server -->
                  <template v-if="ob.isCrypto && false">
                    <div v-html='ob.polyT("listingDetail.inventory", {
                      inventory: `<span class="js-cryptoInventory"></span>`
                    })'>
                    </div>
                  </template>

                  <template v-else-if="ob.metadata.contractType === 'PHYSICAL_GOOD'">
                    <div>
                      {{ ob.polyT('listingDetail.condition', {
                        condition:
                          `<b>${ob.polyT(`conditionTypes.${ob.item.condition.toUpperCase()}`, { _: ob.item.condition })}</b>`
                      }) }}
                    </div>
                  </template>
                </div>
                <hr class="rowLg">
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
                <div class="js-supportedCurrenciesList"></div>
                <template v-if="ob.hasVerifiedMods">
                  <div class="verifiedModBox clrBrAlert2 clrBAlert2Grad">
                    <div class="flexVCent flexHCent gutterHTn rowSm">
                      <div class="badge"
                        :style="`background-image: url(${ob.defaultBadge.tiny}), url('../imgs/verifiedModeratorBadgeDefault.png');`">
                      </div>
                      <div class="tx5 txB">{{ ob.polyT('verifiedMod.modVerified.titleLong') }}</div>
                    </div>
                    <div class="flexColRows gutterVSm tx5b">
                      <div v-html='ob.polyT("verifiedMod.genericDescription", {
                        name: `<b>${ob.verifiedModsData.name}</b>`,
                        link: `<a class="txU noWrap" href="${ob.verifiedModsData.link}" data-open-external>${ob.polyT("verifiedMod.link")}</a>`
                      })'></div>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div>

          <div class="contentBox descriptionSection padLg clrP clrBr clrSh3">
            <h2 class="txUnb">{{ ob.polyT('listingDetail.description') }}</h2>
            <template v-html="ob.item.description"></template>
            <template v-if="!ob.item.description">
              <i class="clrT2">{{ ob.polyT('listingDetail.noDescription') }}</i>
            </template>
          </div>

          <template v-if="ob.item.images.length">
            <div class="contentBox clrSh3 photoSection js-photoSection">
              <div class="flexCent photoSelected js-photoSelected">
                <img class="photoSelectedInner js-photoSelectedInner">
              </div>
              <template v-if="ob.item.images.length > 1">
                <button class="btn ion-ios-arrow-left photoPrev " @click="onClickPhotoPrev"></button>
                <button class="btn ion-ios-arrow-right photoNext " @click="onClickPhotoNext"></button>
              </template>
              <template v-if="ob.item.images.length > 1">
                <div class="photoStrip flex gutterH">
                  <template v-for="(image, index) in ob.item.images">
                    <input type="radio" name="photoStripThumbnails" class="" @click="onClickPhotoSelect"
                      id="photoStrip${index}" :checked="index === 0">
                    <label
                      :style="`background-image: url(` + ob.getServerUrl(`ob/image/${ob.isHiRez() ? image.small : image.tiny}`) + `)`"
                      :for="`photoStrip${index}`"></label>
                  </template>
                </div>
              </template>
            </div>
          </template>
          <div class="js-reviews"></div>

          <!-- Attachments are not yet available -->
          <!--

  <div class="contentBox padLg clrP clrBr clrSh3">
    <h2 class="txUnb">{{ ob.polyT('listingDetail.attachments') }}</h2>
    Placeholder for Attachments
  </div>
 -->

          <template v-if="ob.shippingOptions.length">
            <div class="contentBox padLg clrP clrBr clrSh3" id="shippingSection">
              <h2 class="txUnb">{{ ob.polyT('listingDetail.shipping') }}</h2>
              <div class="flexVCent gutterHLg tx5">
                <!-- this data is not yet available -->
                <!--
        <div>{{ ob.polyT('listingDetail.shipsFrom', { country: `<b>insert translation of the country here</b>` }) }}</div>
        -->
                <div>{{ ob.polyT('listingDetail.shipTo') }}</div>
                <div class="col4">
                  <select id="shippingDestinations" @change="onSetShippingDestination">
                    <option value="ALL">{{ ob.polyT('listingDetail.allCountries') }}</option>
                    <template v-for="country in ob.countryData">
                      <option value="${country.id}" :selected="country.id === ob.defaultCountry">{{ country.text }}
                      </option>
                    </template>
                  </select>
                </div>
              </div>
              <div class="js-shippingOptions">
                <ShippingOptions :options="shippingOptionsInfo" />
              </div>
            </div>
          </template>
          <div class="contentBox padLg clrP clrBr clrSh3">
            <h2 class="txUnb">{{ ob.polyT('listingDetail.refundPolicy') }}</h2>
            <template v-html="ob.refundPolicy"></template>
            <template v-if="!ob.refundPolicy">
              <i class="clrT2">{{ ob.polyT('listingDetail.noRefundPolicy') }}</i>
            </template>
          </div>

          <div class="contentBox padLg clrP clrBr clrSh3">
            <h2 class="txUnb">{{ ob.polyT('listingDetail.termsAndConditions') }}</h2>
            <template v-html="ob.termsAndConditions"></template>
            <template v-if="!ob.termsAndConditions">
              <i class="clrT2">{{ ob.polyT('listingDetail.noTermsAndConditions') }}</i>
            </template>
          </div>

          <div class="js-moreListings"></div>

        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import Backbone, { Collection } from 'backbone';
import bigNumber from 'bignumber.js';
import 'jquery-zoom';
import is from 'is_js';
import app from '../../../../backbone/app';
import 'velocity-animate';
import { getAvatarBgImage } from '../../../../backbone/utils/responsive';
import { convertAndFormatCurrency } from '../../../../backbone/utils/currency';
import { launchEditListingModal } from '../../../../backbone/utils/modalManager';
// import {
//   getInventory,
//   events as inventoryEvents,
// } from '../../../utils/inventory';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { events as outdatedListingHashesEvents } from '../../../../backbone/utils/outdatedListingHashes';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
import BaseModal from '../BaseModal';
import Purchase from '../purchase/Purchase';
import Rating from './Rating';
import Reviews from '../../reviews/Reviews';
import SocialBtns from '../../components/SocialBtns';
// import QuantityDisplay from '../../components/QuantityDisplay';
import { events as listingEvents } from '../../../../backbone/models/listing';
import Listings from '../../../../backbone/collections/Listings';
import PopInMessage, { buildRefreshAlertMessage } from '../../components/PopInMessage';
import { openSimpleMessage } from '../SimpleMessage';
import NsfwWarning from '../NsfwWarning';
import MoreListings from './MoreListings';
import CryptoTradingPair from '../../components/CryptoTradingPair';
import SupportedCurrenciesList from '../../components/SupportedCurrenciesList';

import api from '../../../api';

import ShippingOptions from './ShippingOptions.vue'

export default {
  components: {
    ShippingOptions
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      showModal: true,

      PURCHASE_MODAL_CREATE: 'PURCHASE_MODAL_CREATE',
      PURCHASE_MODAL_DESTROY: 'PURCHASE_MODAL_DESTROY',

      outdateHash: false,

      shippingDestination: '',
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  unmounted() {
    if (this.editModal) this.editModal.remove();
    if (this._purchaseModal) this._purchaseModal.remove();
    if (this.destroyRequest) this.destroyRequest.abort();
    if (this.ratingsFetch) this.ratingsFetch.abort();
    // if (this.inventoryFetch) this.inventoryFetch.abort();
    if (this.moreListingsFetch) this.moreListingsFetch.abort();
  },
  computed: {
    ob () {
      const defaultBadge = app.verifiedMods.defaultBadge(this.model.get('moderators'));

      return {
        ...this.templateHelpers,
        ...this._model,
        shipsFreeToMe: this.shipsFreeToMe,
        ownListing: this.model.isOwnListing,
        price: this.model.price,
        displayCurrency: app.settings.get('localCurrency'),
        // the ships from data doesn't exist yet
        // shipsFromCountry: this.model.get('shipsFrom');
        countryData: this.countryData,
        defaultCountry: this.defaultCountry,
        vendor: this.vendor,
        openedFromStore: this.options.openedFromStore,
        hasVerifiedMods: this.hasVerifiedMods,
        verifiedModsData: app.verifiedMods.data,
        defaultBadge,
        isCrypto: this.model.isCrypto,
        _: { sortBy: _.sortBy },
      };
    },
    templateOptions () {
      const ob = this.ob;

      let tip;
      let buyNowClass = 'disabled';
      let buyNowTranslationKey = ob.metadata.contractType !== 'CRYPTOCURRENCY' ?
        'listingDetail.buyNow' :
        'listingDetail.buyCryptoNow';
      let unpurchaseable = true;

      let coinTypeRateAvailable;
      let cryptoPaymentCoinRateAvailable;

      if (ob.metadata.contractType === 'CRYPTOCURRENCY') {
        coinTypeRateAvailable =
          !!ob.currencyMod.getExchangeRate(ob.item.cryptoListingCurrencyCode);
        cryptoPaymentCoinRateAvailable =
          !!ob.currencyMod.getExchangeRate(ob.metadata.acceptedCurrencies[0]);
      }

      if (!ob.crypto.anySupportedByWallet(ob.metadata.acceptedCurrencies)) {
        tip = ob.polyT('listingDetail.unableToPurchase.incompatibleCrypto',
          {
            acceptedCurs: ob.metadata.acceptedCurrencies.join(', '),
            walletCurs: ob.crypto.supportedWalletCurs()
              .join(', '),
          });
      } else if (
        ob.metadata.contractType !== 'CRYPTOCURRENCY' &&
        !ob.currencyMod.getExchangeRate(ob.price.currencyCode) &&
        !(
          ob.crypto.supportedWalletCurs().includes(ob.price.currencyCode) &&
          ob.metadata.acceptedCurrencies.includes(ob.price.currencyCode)
        )
      ) {
        // If it's priced in a wallet cur and that cur is one of the accepted
        // curs, we won't disable purchase even if there's no exchange rate for the
        // cur because they could still pay for it using that cur making the
        // pricing and payment curs the same and therefore the exchange rate
        // unnecessary.
        tip = ob.polyT('listingDetail.unableToPurchase.noExchangeRateInfo', {
          cur: ob.price.currencyCode
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

      return { tip, buyNowClass, buyNowTranslationKey, unpurchaseable }
    },

    shippingOptionsInfo() {
      const shippingOptions = this._model.shippingOptions;
      const templateData = shippingOptions.filter((option) => {
        if (this.shippingDestination === 'ALL') return option.regions;
        return option.regions.includes(this.shippingDestination);
      });

      return {
        templateData,
        displayCurrency: app.settings.get('localCurrency'),
        pricingCurrency: this.model.price.currencyCode,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      if (!options.model) {
        throw new Error('Please provide a model.');
      }

      const opts = {
        checkNsfw: true,
        removeOnClose: true,
        ...options,
      };

      this.baseInit(opts);

      this._shipsFreeToMe = this.model.shipsFreeToMe;
      this.activePhotoIndex = 0;

      // Set to an empty bigNumber instance so if we can't fill it with a legitmate
      // value, at least bigNumber ops won't fail.
      this.totalPrice = bigNumber();

      try {
        this.totalPrice = this.model.get('item').get('price');
      } catch (e) {
        // pass
      }

      this._purchaseModal = null;
      this._latestHash = this.model.get('hash');
      this._renderedHash = null;

      // Sometimes a profile model is available and the vendor info
      // can be obtained from that.
      if (opts.profile) {
        this.vendor = {
          peerID: opts.profile.id,
          name: opts.profile.get('name'),
          handle: opts.profile.get('handle'),
          avatarHashes: opts.profile.get('avatarHashes').toJSON(),
        };
      }

      // In most cases the page opening this modal will already have and be able
      // to provide the vendor information. If it cannot, then I suppose we
      // could fetch the profile and lazy load it in, but we can cross that
      // bridge when we get to it.
      this.vendor = this.vendor || opts.vendor;

      this.countryData = getTranslatedCountries()
        .map((countryObj) => ({ id: countryObj.dataName, text: countryObj.name }));

      this.defaultCountry = app.settings.get('shippingAddresses').length
        ? app.settings.get('shippingAddresses').at(0).get('country') : app.settings.get('country');

      this.listenTo(app.settings, 'change:country', () => (this.shipsFreeToMe = this.model.shipsFreeToMe));

      this.listenTo(
        app.settings.get('shippingAddresses'),
        'update',
        (cl, updateOpts) => {
          if (updateOpts.changes.added.length
            || updateOpts.changes.removed.length) {
            this.shipsFreeToMe = this.model.shipsFreeToMe;
          }
        },
      );

      this.listenTo(this.model, 'someChange', () => this.showDataChangedMessage());

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

            if (!(_.isEqual(prev, cur))) {
              this.showDataChangedMessage();
            }
          }
        });

        this.listenTo(app.profile.get('avatarHashes'), 'change', () => {
          this.$storeOwnerAvatar
            .attr('style', getAvatarBgImage(app.profile.get('avatarHashes').toJSON()));
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

      this.rating = this.createChild(Rating);

      // get the ratings data, if any
      this.ratingsFetch = $.get(app.getServerUrl(`ob/ratingindex/${this.vendor.peerID}/${this.model.get('slug')}`))
        .done((data) => this.onRatings(data))
        .fail((jqXhr) => {
          if (jqXhr.statusText === 'abort') return;
          const failReason = jqXhr.responseJSON && jqXhr.responseJSON.reason || '';
          openSimpleMessage(
            app.polyglot.t('listingDetail.errors.fetchRatings'),
            failReason,
          );
        });

      this.reviews = this.createChild(Reviews, {
        async: true,
        showListingData: true,
        initialState: {
          isFetchingRatings: true,
        },
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

      const fetchOpts = this.vendor.peerID === app.profile.id ? {}
        : {
          data: $.param({
            'max-age': 60 * 60, // 1 hour
          }),
        };

      this.moreListingsFetch = this.moreListingsCol.fetch(fetchOpts)
        .done(() => {
          this.moreListingsData = this.randomizeMoreListings(this.moreListingsCol);
          setTimeout(() => {
            if (this.moreListings) {
              this.moreListings.setState({
                listings: this.moreListingsData,
              });
            }
          });
        });

      this.rendered = false;
      this._outdatedHashState = null;
    },

    events () {
      return {
        'click .js-reloadOutdated': 'onClickReloadOutdated',
      };
    },

    onDocumentClick () {
      this.$deleteConfirmedBox.addClass('hide');
    },

    onRatings (data) {
      const pData = data || {};
      this.rating.averageRating = pData.average;
      this.rating.ratingCount = pData.count;
      this.rating.fetched = true;
      this.rating.render();
      this.reviews.reviewIDs = pData.ratings || [];
      this.reviews.setState({ isFetchingRatings: false });
    },

    onClickEditListing () {
      recordEvent('Listing_EditFromListing');
      const onCloseEditModal = () => {
        this.showModal = false;

        if (!this.isRemoved()) {
          this.showModal = true;
        }
      };

      const onEditModalClickReturn = () => {
        this.editModal.confirmClose()
          .done(() => {
            this.stopListening(null, null, onCloseEditModal);
            this.editModal.remove();

            this.showModal = true;
          });
      };

      this.editModal = launchEditListingModal({
        model: this.model,
        returnText: app.polyglot.t('listingDetail.editListingReturnText'),
        onClickViewListing: onEditModalClickReturn,
      });

      this.showModal = false;
      this.listenTo(this.editModal, 'close', onCloseEditModal);
      this.listenTo(this.editModal, 'click-return', onEditModalClickReturn);
    },

    onClickCloneListing () {
      recordEvent('Listing_CloneFromListing');
      launchEditListingModal({
        model: this.model.cloneListing(),
      });
    },

    onClickDeleteListing () {
      recordEvent('Listing_DeleteFromListing');
      this.$deleteConfirmedBox.removeClass('hide');
      // don't bubble to the document click handler
      return false;
    },

    onClickDeleteConfirmBox () {
      // don't bubble to the document click handler
      return false;
    },

    onClickConfirmedDelete () {
      recordEvent('Listing_DeleteFromListingConfirm');
      if (this.destroyRequest && this.destroyRequest.state === 'pending') return;
      this.destroyRequest = this.model.destroy({ wait: true });

      if (this.destroyRequest) {
        this.$deleteListing.addClass('processing');

        this.destroyRequest.done(() => {
          if (this.destroyRequest.statusText === 'abort'
            || this.isRemoved()) return;

          this.close();
        }).always(() => {
          if (!this.isRemoved()) {
            this.$deleteListing.removeClass('processing');
          }
        });
      }
    },

    onClickConfirmCancel () {
      recordEvent('Listing_DeleteFromListingCancel');
      this.$deleteConfirmedBox.addClass('hide');
    },

    onClickGotoPhotos () {
      recordEvent('Listing_GoToPhotos', { ownListing: this.model.isOwnListing });
      this.gotoPhotos();
    },

    onClickGoToStore () {
      if (this.options.openedFromStore) {
        recordEvent('Listing_GoToStore',
          {
            OpenedFromStore: true,
            ownListing: this.model.isOwnListing,
          });
        this.close();
      } else {
        recordEvent('Listing_GoToStore',
          {
            OpenedFromStore: false,
            ownListing: this.model.isOwnListing,
          });
        const base = this.vendor.handle ? `@${this.vendor.handle}` : this.vendor.peerID;
        app.router.navigateUser(`${base}/store`, this.vendor.peerID, { trigger: true });
      }
    },

    randomizeMoreListings (cl) {
      if (!(cl instanceof Collection)) {
        throw new Error('Please provide a Collection instance.');
      }

      return _.shuffle(cl.models)
        .filter((md) => md.get('slug') !== this.model.get('slug'))
        .map((md) => md.toJSON())
        .slice(0, 8);
    },

    gotoPhotos () {
      recordEvent('Listing_GoToPhotos', { ownListing: this.model.isOwnListing });
      this.$photoSection.velocity(
        'scroll',
        {
          duration: 500,
          easing: 'easeOutSine',
          container: this.$el,
        });
    },

    clickRating () {
      recordEvent('Listing_ClickOnRatings', { ownListing: this.model.isOwnListing });
      this.gotoReviews();
    },

    gotoReviews () {
      this.$reviews.velocity(
        'scroll',
        {
          duration: 500,
          easing: 'easeOutSine',
          container: this.$el,
        });
    },

    onClickPhotoSelect (e) {
      recordEvent('Listing_ClickOnPhoto', { ownListing: this.model.isOwnListing });
      this.setSelectedPhoto($(e.target).index('.js-photoSelect'));
    },

    setSelectedPhoto (photoIndex) {
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
      this.$photoSelected.trigger('zoom.destroy'); // old zoom must be removed
      this.$photoSelectedInner.attr('src', phSrc);
    },

    activateZoom () {
      if (this.$photoSelectedInner.width() >= this.$photoSelected.width()
        || this.$photoSelectedInner.height() >= this.$photoSelected.height()) {
        this.$photoSelected
          .removeClass('unzoomable')
          .zoom({
            url: this.$photoSelectedInner.attr('src'),
            on: 'click',
            onZoomIn: () => {
              this.$photoSelected.addClass('open');
            },
            onZoomOut: () => {
              this.$photoSelected.removeClass('open');
            },
          });
      } else {
        this.$photoSelected.addClass('unzoomable');
      }
    },

    setActivePhotoThumbnail (thumbIndex) {
      if (is.not.number(thumbIndex)) {
        throw new Error('Please provide an index for the selected photo thumbnail.');
      }
      if (thumbIndex < 0) {
        throw new Error('Please provide a valid index for the selected photo thumbnail.');
      }
      this.$photoRadioBtns.prop('checked', false).eq(thumbIndex).prop('checked', true);
    },

    onClickPhotoPrev () {
      recordEvent('Listing_ClickOnPhotoPrev', { ownListing: this.model.isOwnListing });
      let targetIndex = this.activePhotoIndex - 1;
      const imagesLength = parseInt(this.model.toJSON().item.images.length, 10);

      targetIndex = targetIndex < 0 ? imagesLength - 1 : targetIndex;
      this.setSelectedPhoto(targetIndex);
      this.setActivePhotoThumbnail(targetIndex);
    },

    onClickPhotoNext () {
      recordEvent('Listing_ClickOnPhotoNext', { ownListing: this.model.isOwnListing });
      let targetIndex = this.activePhotoIndex + 1;
      const imagesLength = parseInt(this.model.toJSON().item.images.length, 10);

      targetIndex = targetIndex >= imagesLength ? 0 : targetIndex;
      this.setSelectedPhoto(targetIndex);
      this.setActivePhotoThumbnail(targetIndex);
    },

    onClickFreeShippingLabel () {
      recordEvent('Listing_ClickFreeShippingLabel', { ownListing: this.model.isOwnListing });
      this.gotoShippingOptions();
    },

    gotoShippingOptions () {
      this.$shippingSection.velocity(
        'scroll',
        {
          duration: 500,
          easing: 'easeOutSine',
          container: this.$el,
        });
    },

    onChangeVariantSelect () {
      this.adjustPriceBySku();
    },

    adjustPriceBySku () {
      const variantCombo = [];
      // assemble a combo of the indexes of the selected variants
      this.variantSelects.each((i, select) => {
        variantCombo.push($(select).prop('selectedIndex'));
      });

      const { options } = this.model.toJSON().item;
      const selections = variantCombo.map((val, idx) => ({
        option: options[idx].name,
        variant: options[idx].variants[val].name,
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
          let adjPrice = '';

          try {
            adjPrice = convertAndFormatCurrency(
              this.totalPrice,
              this.model
                .get('metadata')
                .get('pricingCurrency')
                .code,
              app.settings.get('localCurrency')
            );
          } catch (e) {
            // pass
            console.error(e);
          }

          $('.js-price').html(adjPrice);
        }
      } catch (e) {
        // pass
      }
    },

    showDataChangedMessage () {
      if (this.dataChangePopIn && !this.dataChangePopIn.isRemoved()) {
        this.dataChangePopIn.$el.velocity('callout.shake', { duration: 500 });
      } else {
        this.dataChangePopIn = this.createChild(PopInMessage, {
          messageText:
            buildRefreshAlertMessage(app.polyglot.t('listingDetail.listingDataChangedPopin')),
        });

        this.listenTo(this.dataChangePopIn, 'clickRefresh', () => (this.render()));

        this.listenTo(this.dataChangePopIn, 'clickDismiss', () => {
          this.dataChangePopIn.remove();
          this.dataChangePopIn = null;
        });

        this.$popInMessages.append(this.dataChangePopIn.render().el);
      }
    },

    onClickReloadOutdated () {
      let defaultPrevented = false;

      this.trigger('clickReloadOutdated', {
        preventDefault: () => (defaultPrevented = true),
      });

      setTimeout(() => {
        if (!defaultPrevented) {
          Backbone.history.loadUrl();
        }
      });
    },

    onSetShippingDestination (val) {
      this.shippingDestination = val;
    },

    /**
     * Returns a promise that will fire progress notifications when a purchase modal
     * is created. Will also fire a notifications when one is destroyed.
     */
    get purchaseModal () {
      this._purchaseModalDeferred = this._purchaseModalDeferred || $.Deferred();

      if (this._purchaseModal) {
        this._purchaseModalDeferred.notify({
          type: this.PURCHASE_MODAL_CREATE,
          view: this._purchaseModal,
        });
      }

      return this._purchaseModalDeferred.promise();
    },

    startPurchase () {
      if (!this.model.isCrypto) {
        if (this.totalPrice.lte(0)) {
          openSimpleMessage(
            app.polyglot.t('listingDetail.errors.noPurchaseTitle'),
            app.polyglot.t('listingDetail.errors.zeroPriceMsg'),
          );
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

      const selectedVariants = this.getSelectedVariants();

      if (this._purchaseModal) this._purchaseModal.remove();

      this._purchaseModal = new Purchase({
        listing: this.model,
        variants: selectedVariants,
        vendor: this.vendor,
        removeOnClose: true,
        showCloseButton: false,
        phase: 'pay',
        // inventory: this._inventory,
      })
        .render()
        .open();

      if (this._purchaseModalDeferred) {
        this._purchaseModalDeferred.notify({
          type: this.PURCHASE_MODAL_CREATE,
          view: this._purchaseModal,
        });
      }

      this._purchaseModal.on('modal-will-remove', () => {
        this._purchaseModal = null;
        if (this._purchaseModalDeferred) {
          this._purchaseModalDeferred.notify({
            type: this.PURCHASE_MODAL_DESTROY,
          });
        }
      });

      this.listenTo(this._purchaseModal, 'closeBtnPressed', () => this.close());
      recordEvent('Purchase_Start', { ownListing: this.model.isOwnListing });
    },

    getSelectedVariants () {
      const selectedVariants = [];
      this.variantSelects.each((i, select) => {
        const variant = {};
        variant.name = $(select).attr('name');
        variant.value = $(select).val();
        selectedVariants.push(variant);
      });

      return selectedVariants;
    },

    addToCart () {
      const selectedVariants = this.getSelectedVariants();

      api.addToShoppingCart(this.vendor.peerID, {
        slug: this.model.get('slug'),
        quantity: '1',
        options: selectedVariants || [],
      })
    },

    get shipsFreeToMe () {
      return this._shipsFreeToMe;
    },

    set shipsFreeToMe (shipsFree) {
      const prevVal = this._shipsFreeToMe;
      this._shipsFreeToMe = !!shipsFree;

      if (prevVal !== this._shipsFreeToMe) {
        this.$shipsFreeBanner[this._shipsFreeToMe ? 'removeClass' : 'addClass']('hide');
      }
    },

    get $deleteListing () {
      return this._$deleteListing || $('.js-deleteListing');
    },

    get $shipsFreeBanner () {
      return this._$shipsFreeBanner || $('.js-shipsFreeBanner');
    },

    get $popInMessages () {
      return this._$popInMessages
        || (this._$popInMessages = $('.js-popInMessages'));
    },

    get $photoSection () {
      return this._$photoSection
        || (this._$photoSection = $('.js-photoSection'));
    },

    get $photoSelected () {
      return this._$photoSelected
        || (this._$photoSelected = $('.js-photoSelected'));
    },

    get $shippingSection () {
      return this._$shippingSection
        || (this._$shippingSection = $('#shippingSection'));
    },

    get $photoRadioBtns () {
      return this._$photoRadioBtns
        || (this._$photoRadioBtns = $('.js-photoSelect'));
    },

    get $storeOwnerAvatar () {
      return this._$storeOwnerAvatar
        || (this._$storeOwnerAvatar = $('.js-storeOwnerAvatar'));
    },

    get $deleteConfirmedBox () {
      return this._$deleteConfirmedBox
        || (this._$deleteConfirmedBox = $('.js-deleteConfirmedBox'));
    },

    render () {
      if (this.dataChangePopIn) this.dataChangePopIn.remove();

      let nsfwWarning;

      if (!this.rendered
        && this.options.checkNsfw
        && this.model.get('item').get('nsfw')
        && !this.model.isOwnListing && !app.settings.get('showNsfw')) {
        nsfwWarning = new NsfwWarning()
          .render()
          .open();
        this.listenTo(nsfwWarning, 'canceled', () => this.close());
      }

      if (nsfwWarning) this.showModal = false;

      $('.js-rating').append(this.rating.render().$el);
      this.$reviews = $('.js-reviews');
      this.$reviews.append(this.reviews.render().$el);

      if (this._latestHash !== this.model.get('hash')) {
        this.outdateHash = true;
      }

      if (this.supportedCurrenciesList) this.supportedCurrenciesList.remove();
      this.supportedCurrenciesList = this.createChild(SupportedCurrenciesList, {
        initialState: {
          currencies: this.model.get('metadata')
            .get('acceptedCurrencies'),
        },
      });
      $('.js-supportedCurrenciesList')
        .append(this.supportedCurrenciesList.render().el);

      if (!this.model.isOwnListing) {
        if (this.socialBtns) this.socialBtns.remove();
        this.socialBtns = this.createChild(SocialBtns, {
          targetID: this.vendor.peerID,
        });
        $('.js-socialBtns').append(this.socialBtns.render().$el);
      }

      if (this.moreListings) this.moreListings.remove();
      this.moreListings = this.createChild(MoreListings, {
        initialState: {
          vendor: this.vendor,
          listings: this.moreListingsData,
        },
      });
      this.listenTo(this.moreListings, 'listingDetailOpened', () => this.remove());
      $('.js-moreListings')
        .append(this.moreListings.render().$el);

      this.$photoSelectedInner = $('.js-photoSelectedInner');
      this._$deleteListing = null;
      this._$shipsFreeBanner = null;
      this._$popInMessages = null;
      this._$photoSection = null;
      this._$photoSelected = null;
      this._$photoRadioBtns = null;
      this._$shippingSection = null;
      this._$storeOwnerAvatar = null;
      this._$deleteConfirmedBox = null;

      this.$photoSelectedInner.on('load', () => this.activateZoom());

      this.variantSelects = $('.js-variantSelect');

      this.variantSelects.select2({
        // disables the search box
        minimumResultsForSearch: Infinity,
      });

      $('#shippingDestinations').select2();
      this.shippingDestination = this.defaultCountry;

      this.setSelectedPhoto(this.activePhotoIndex);
      this.setActivePhotoThumbnail(this.activePhotoIndex);

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

        if (this.cryptoTitle) this.cryptoTitle.remove();
        this.cryptoTitle = this.createChild(CryptoTradingPair, {
          initialState: {
            tradingPairClass: 'cryptoTradingPairXL rowSm',
            exchangeRateClass: 'clrT2 exchangeRateLine',
            fromCur: metadata.get('acceptedCurrencies')[0],
            toCur: this.model.get('item').get('cryptoListingCurrencyCode'),
          },
        });
        $('.js-cryptoTitle')
          .html(this.cryptoTitle.render().el);
      } else {
        this.adjustPriceBySku();
      }

      if (nsfwWarning) {
        setTimeout(() => {
          nsfwWarning.bringToTop();
          this.showModal = true;
        });
      }

      this.rendered = true;
      this._renderedHash = this.model.get('hash');

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
