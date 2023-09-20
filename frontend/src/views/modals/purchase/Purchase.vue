<template>
  <div class="modal purchase modalScrollPage">
    <BaseModal :modalInfo="{ removeOnClose: true, showCloseButton: false }">
      <template v-slot:component>
        <div class="popInMessageHolder js-popInMessages"></div>

        <div class="topControls gutterHSm flex">
          <div v-if="vendor">
            <div class="contentBox clrP clrSh3 clrBr clrT">
              <div class="padSm gutterHSm overflowAuto margRSm flexVCent">
                <a class="clrBr2 clrSh1 discTn flexNoShrink" :style="ob.getAvatarBgImage(vendor.avatarHashes)"></a>
                <p class="txUnl tx3 clamp">{{ vendor.name }}</p>
                <a class="link flexNoShrink tx6" @click="clickGoToListing">{{ ob.polyT('purchase.returnToListing') }}</a>
              </div>
            </div>
          </div>
        </div>

        <div :class="`flexRow gutterH mainSection ${`phase${capitalize(phase)}`}`">
          <div class="col9">
            <div class="flexColRow gutterV">
              <div v-for="(listing, key) in listings" :key="key">
                <section class="contentBox pad clrP clrBr clrSh3">
                  <div class="js-errors"></div>
                  <div class="js-items-quantity-errors"></div>
                  <div v-if="!listing.isCrypto">
                    <div class="flexVCent gutterH">
                      <div class="thumb" :style="ob.getListingBgImage(listing.listing.item.images[0])"></div>
                      <div class="flexExpand">
                        <div class="flexCol gutterVTn">
                          <div class="width100 noOverflow">
                            <b>{{ listing.listing.item.title }}</b>
                          </div>
                          <div v-for="(variant, j) in listing.options" :key="j">
                            <div class="width100 noOverflow">
                              <span class="clrT2">{{ variant.name }}: {{ variant.value }}</span>
                            </div>
                          </div>
                        </div>
                      </div>
                      <div v-if="phase === 'pay' || phase === 'processing'">
                        <div class="flexNoShrink">
                          <div class="flexVCent gutterH">
                            <div class="flexCol">
                              <label class="flexHRight" for="purchaseQuantity">
                                <span class="required txB margR">{{ ob.polyT('purchase.quantity') }}</span>
                              </label>
                            </div>
                            <div class="flexNoShrink">
                              <input
                                class="clrBr clrP"
                                type="text"
                                id="purchaseQuantity"
                                size="3"
                                name="quantity"
                                :value="listing.quantity"
                                @keyup="keyupQuantity"
                                placeholder="0"
                                data-var-type="bignumber">
                            </div>
                          </div>
                        </div>
                      </div>
                      <div class="pad flexNoShrink"><b>{{ listing.price }}</b></div>
                    </div>
                  </div>

                  <div v-else>
                    <div class="flexVCent gutterHLg row cryptoTitleWrap">
                      <div :class="`js-cryptoTitle ${phase !== 'pay' && phase !== 'processing' ? 'flexExpand' : ''}`"></div>
                      <div v-if="phase === 'pay' || phase === 'processing'">
                        <div class="flexExpand">
                          <div class="flexVCent gutterHLg">
                            <label for="cryptoAmount" class="clrT txB required">{{ ob.polyT('purchase.cryptoAmount') }}</label>
                            <div class="inputSelect">
                              <input
                                type="text"
                                class="clrBr clrP clrSh2"
                                name="quantity"
                                id="cryptoAmount"
                                @change="onChangeCryptoAmount"
                                :value="uiQuantity"
                                @keyup="keyupQuantity"
                                placeholder="0.0000"
                                size="8"
                                data-var-type="bignumber">
                              <div v-if="app.settings.get('localCurrency') !== listing.listing.item.cryptoListingCurrencyCode">
                                <select
                                  id="cryptoAmountCurrency"
                                  @change="changeCryptoAmountCurrency"
                                  class="clrBr clrP nestInputRight">
                                  <option
                                    v-for="(cur, j) in [listing.listing.item.cryptoListingCurrencyCode, app.settings.get('localCurrency')]"
                                    :key="j"
                                    :value="cur"
                                    :selected="cur === cryptoAmountCurrency">{{ cur }}</option>
                                </select>
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                      <div class="pad flexNoShrink">
                      {{ 
                        ob.crypto.cryptoPrice({
                          priceAmount: prices[0].price.plus(prices[0].vPrice),
                          priceCurrencyCode: listing.price.currencyCode,
                          displayCurrency: displayCurrency,
                          priceModifier: listing.listing.item.cryptoListingPriceModifier,
                        })
                      }}
                      </div>
                    </div>
                    <hr class="clrBr rowLg" />
                    <div class="rowSm">
                      <label class="h4 flexExpand required" for="purchaseCryptoAddress">{{ heading }}</label>
                    </div>
                    <div class="js-items-paymentAddress-errors"></div>
                    <div v-if="phase === 'pay' || phase === 'processing'">
                      <input type="text"
                        id="purchaseCryptoAddress"
                        @change="changeCryptoAddress"
                        :value="items.length > 0 ?items[0].paymentAddress : ''"
                        :placeholder="ob.polyT('purchase.cryptoAddressPlaceholder', { coinType: coinName})"
                        class="clrBr clrP rowSm"
                        :maxlength="itemConstraints.maxPaymentAddressLength" />
                    </div>

                    <div v-else>
                      <p class="cryptoPaymentAddress">{{ items.length > 0 ?items[0].paymentAddress : '' }}</p>
                    </div>
                    <div class="txSm clrT2">{{ helper }}</div>
                  </div>
                </section>
              </div>
            </div>
            <div v-if="phase === 'pay' || phase === 'processing'">
              <div v-if="listing && listing.toJSON && listing.toJSON().shippingOptions && listing.toJSON().shippingOptions.length">
                <section class="contentBox padMd clrP clrBr clrSh3 js-shipping">
                  <div class="js-shipping-errors js-items-shipping-errors"></div>
                  <Shipping
                    v-if="listing.get('shippingOptions').length"
                    :listing="listing"
                    @shippingOptionSelected="updateShippingOption"
                    @newAddress="clickNewAddress"
                    />
                </section>
              </div>
              <section class="contentBox padMd clrP clrBr clrSh3">
                <div class="flexColRows gutterVSm">
                  <div>
                    <div class="js-paymentCoin-errors"></div>
                    <h2 class="h4 flexExpand required">{{ ob.polyT('purchase.cryptoCurrencyTitle') }}</h2>
                    <div class="js-cryptoCurSelectorWrapper"></div>
                  </div>
                </div>
              </section>
              <section class="contentBox padMd clrP clrBr clrSh3">
                <div class="flexColRows gutterVSm">
                  <div class="flexVCentClearMarg">
                    <h2 class="h4 flexExpand required">{{ ob.polyT('purchase.paymentTypeTitle') }}</h2>
                    <div v-if="showModerators">
                      <input type="checkbox" id="purchaseVerifiedOnly" @click="onClickVerifiedOnly" :checked="showVerifiedOnly">
                      <label class="tx5b" for="purchaseVerifiedOnly">{{ ob.polyT('settings.storeTab.verifiedOnly') }}</label>
                    </div>
                  </div>
                  <div v-if="showModerators && !ob.noValidModerators">
                    <div class="js-moderated-errors"></div>
                    <div class="js-moderatorsWrapper"></div>
                    <div v-if="!ob.noValidModerators">
                      <div>
                        <div class="clrT2 tx6 rowMd">{{ ob.polyT('purchase.moderatorsDisclaimer') }}</div>
                      </div>
                    </div>
                    <hr class="clrBr row">
                  </div>
                  <DirectPayment class="moderatorsList" :active="!isModerated" @click="handleDirectPurchaseClick" />
                </div>
              </section>
              <section class="contentBox padMd clrP clrBr clrSh3">
                <h2 class="h4">
                  {{ ob.polyT('purchase.informationTitle') }}
                  <span class="clrT2 txUnb tx5b">{{ ob.polyT('purchase.optional') }}</span>
                </h2>
                <div class="flexRow gutterH row">
                  <div class="col6">
                    <div class="rowTn">
                      <label for="emailAddress" class="tx5">
                        {{ ob.polyT('purchase.emailAddress') }}
                      </label>
                    </div>
                    <div>
                      <input
                        class="btnHeight clrBr clrP js-purchaseField"
                        type="text"
                        id="emailAddress"
                        name="alternateContactInfo"
                        @blur="blurEmailAddress"
                        :placeholder="ob.polyT('purchase.emailPlaceholder')"
                        :value="order.alternateContactInfo">
                    </div>
                    <div>
                      <span class="txSm clrT2">{{ ob.polyT('purchase.emailNote') }}</span>
                    </div>
                  </div>
                  <div class="col6">
                    <div v-if="hasCoupons">
                      <div class="rowTn">
                        <label for="couponCode" class="tx5">
                          {{ ob.polyT('purchase.couponCode') }}
                        </label>
                      </div>
                      <div class="flex gutterH row">
                        <input
                          class="btnHeight clrBr clrP"
                          type="text"
                          id="couponCode"
                          @keyup="onKeyUpCouponCode"
                          name="couponCode"
                          :placeholder="ob.polyT('purchase.couponCodePlaceholder')">
                        <button class="btn clrP clrBr clrSh2 flexNoShrink" @click="applyCoupon">
                          {{ ob.polyT('purchase.applyCode') }}
                        </button>
                      </div>
                      <div class="js-couponsWrapper">
                        <!-- // coupons are inserted here after they are added by the user. -->
                      </div>
                    </div>
                  </div>
                </div>
                <hr class="clrBr row">
                <div class="rowTn">
                  <label for="memo" class="tx5">
                    {{ ob.polyT('purchase.memo') }}
                  </label>
                </div>
                <textarea
                  class="clrBr clrP js-purchaseField"
                  id="memo"
                  name="memo"
                  @blur="blurMemo"
                  maxlength="5000"
                  rows="6"
                  :placeholder="ob.polyT('purchase.memoPlaceholder')">{{ items.length > 0 ? items[0].memo : '' }}</textarea>
              </section>
            </div>
            <div v-if="phase === 'pending'">
              <section class="contentBox padMd clrP clrBr clrSh3 js-pending"></section>
            </div>
            <div v-if="phase === 'complete'">
              <section class="contentBox padMd clrP clrBr clrSh3 js-complete"></section>
            </div>
          </div>
          <div class="col3">
            <section class="contentBox pad clrP clrBr clrSh3 sidebar">
              <i class="cornerTR ion-ios-close-empty iconBtn clrP clrBr clrSh3 closeBtn" @click="clickClose"></i>
              <div class="js-actionBtn"></div>
              <div class="rowLg">
                <!-- <div class="js-receipt"></div> -->
                <Receipt :order="order" :listing="listing" :prices="prices" :coupons="couponObj" :paymentCoin="paymentCoin" :showTotalTip="phase === 'pay'"/>
                <div v-if="showModerators && !ob.noValidModerators">
                  <hr class="clrBr">
                  <div class="padSm txSm txCtr clrT2">
                    {{ ob.polyT('purchase.moderatorNote') }}
                  </div>
                </div>
              </div>
              <div class="tx6 js-feeChangeContainer"></div>
            </section>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import Backbone from 'backbone';
import bigNumber from 'bignumber.js';
import 'velocity-animate';
import { ERROR_DUST_AMOUNT } from '../../../../backbone/constants';
import { removeProp } from '../../../../backbone/utils/object';
import app from '../../../../backbone/app';
import loadTemplate from '../../../../backbone/utils/loadTemplate';
import { launchSettingsModal } from '../../../../backbone/utils/modalManager';
// import {
//   getInventory,
//   events as inventoryEvents,
// } from '../../../utils/inventory';
import { startAjaxEvent, endAjaxEvent } from '../../../../backbone/utils/metrics';
import { toStandardNotation } from '../../../../backbone/utils/number';
import {
  decimalToInteger,
  isValidCoinDivisibility,
  curDefToDecimal,
  getCoinDivisibility,
} from '../../../../backbone/utils/currency';
import { capitalize } from '../../../../backbone/utils/string';
import { events as outdatedListingHashesEvents } from '../../../../backbone/utils/outdatedListingHashes';
import { isSupportedWalletCur } from '../../../../backbone/data/walletCurrencies';
import Order from '../../../../backbone/models/purchase/Order';
import Item from '../../../../backbone/models/purchase/Item';
import Listing from '../../../../backbone/models/listing/Listing';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import PopInMessage, { buildRefreshAlertMessage } from '../../../../backbone/views/components/PopInMessage';
import Moderators from '../../../../backbone/views/components/moderators/Moderators';
import FeeChange from '../../../../backbone/views/components/FeeChange';
import CryptoTradingPair from '../../../../backbone/views/components/CryptoTradingPair';
import CryptoCurSelector from '../../../../backbone/views/components/CryptoCurSelector';
import Shipping from './Shipping.vue';
import Receipt from './Receipt.vue';
import Coupons from '../../../../backbone/views/modals/purchase/Coupons';
import ActionBtn from '../../../../backbone/views/modals/purchase/ActionBtn';
import Payment from '../../../../backbone/views/modals/purchase/Payment';
import Complete from '../../../../backbone/views/modals/purchase/Complete';
import DirectPayment from './DirectPayment.vue';

import { toRaw } from 'vue';

export default {
  components: {
    Receipt,
    DirectPayment,
    Shipping,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  mixins: [],
  data () {
    return {
      phase: 'pay',
      cart: {},
      ob: {},
      vendor: {},
      order: new Order({}, {}),
      items: [],

      listing: undefined,
      listings: [],
      moderators: undefined,
      couponObj: [],
      cryptoCurSelector: undefined,

      coinName: '',
      moderatorIDs: [],
      showVerifiedOnly: true,
      shipping: {
        selectedAddress: '',
      }
    };
  },
  created () {
    this.initEventChain();
  },
  mounted () {
    this.loadData(this.$store.state.cart);
  },
  computed: {
    helperMessage () {
      const warning = this.phase === 'pay' || this.phase === 'processing' ?
        `<b>${ob.polyT('purchase.cryptoAddressHelperWarning')}</b>` :
        `<b>${ob.polyT('purchase.cryptoAddressHelperWarning2')}</b>`;

      return ob.polyT('purchase.cryptoAddressHelper', {
        name: this.vendor.name,
        coinType: this.coinName,
        warning,
      });
    },
    showModerators () {
      return this.moderatorIDs.length;
    },
    displayCurrency() {
      return app.settings.get('localCurrency');
    },
    paymentCoin () {
      return this.cryptoCurSelector ? this.cryptoCurSelector.getState().activeCurs[0] : '';
    },
    prices () {
      // return an array of price objects that matches the items in the order
      return this.order.get('items').map((item) => {
        const shipping = item.get('shipping');
        const sName = shipping.get('name');
        const sService = shipping.get('service');
        const sOpt = this.listing.get('shippingOptions').findWhere({ name: sName });
        const sOptService = sOpt ? sOpt.get('services').findWhere({ name: sService }) : '';

        const options = item.get('options');
        const selections = options.map((option) => ({
          option: option.name,
          variant: option.value,
        }));
        const sku = this.listing.get('item').get('skus').find((v) => _.isEqual(v.get('selections'), selections));

        return {
          price: bigNumber(this.listing.price.amount),
          sPrice: bigNumber(sOptService ? sOptService.get('price') || 0 : 0),
          aPrice: bigNumber(sOptService ? sOptService.get('additionalItemPrice') || 0 : 0),
          vPrice: bigNumber(sku ? sku.get('surcharge') || 0 : 0),
          quantity: bigNumber(item.get('quantity')),
        };
      });
    },
    couponField () {
      if (!this._couponField) {
        this._couponField = this.$('#couponCode');
      }
      return this._couponField;
    },
    cryptoAmountCurrency () {
      return this._cryptoAmountCurrency
        || this.listing.get('item')
          .get('cryptoListingCurrencyCode');
    },

    coinDivisibility () {
      let currencyCode;
      try {
        currencyCode = this.listing.isCrypto ? this.listing.get('item').cryptoListingCurrencyCode : this.listing.get('metadata').get('pricingCurrency').code;
      } catch (e) {
        // pass
      }

      let coinDiv;
      try {
        coinDiv = getCoinDivisibility(currencyCode);
      } catch (e) {
        // pass
      }
      return coinDiv;
    },

    isModerated () {
      return this.moderators ? this.moderators.selectedIDs.length > 0 : false;
    },

    itemConstraints () {
      return this.order.get('items')
            .at(0)
            .constraints;
    },

    hasCoupons () {
      return this.listing && this.listing.get('coupons').length
            && this.listing.get('metadata').get('contractType') !== 'CRYPTOCURRENCY';
    }
  },
  methods: {
    capitalize,

    async loadData (opts = {}) {
      this.cart = toRaw(opts.cart);

      this.listings = [];
      this.cart.items.forEach(item => {
        console.log("item: ", item)
        this.listings.push(item)
      });

      let input = this.cart.items[0];
      this.listing = new Listing({ slug: input.listing.slug, }, { guid: this.cart.vendorID });
      await this.listing.fetch();

      // const coinType = this.listing.listing.item.cryptoListingCurrencyCode;
      // const coinTranslationKey = `cryptoCurrencies.${coinType}`;
      // this.coinName = ob.polyT(coinTranslationKey) === coinTranslationKey ? coinType : ob.polyT(coinTranslationKey);

      this.vendor = {
        peerID: this.cart.vendorID,
        name: this.cart.profile.name,
        handle: this.cart.profile.handle,
        avatarHashes: this.cart.profile.avatarHashes,
      };

      this.variants = this.cart.items[0].options;

      const shippingOptions = this.listing.get('shippingOptions');
      const moderatorIDs = this.listing.get('moderators') || [];
      const disallowedIDs = [app.profile.id, this.listing.get('vendorID').peerID];
      this.moderatorIDs = _.without(moderatorIDs, ...disallowedIDs);

      this.couponObj = [];

      this.order = new Order(
        {},
        {
          shippable: !!(shippingOptions && shippingOptions.length),
          moderated: this.moderatorIDs.length && app.verifiedMods.matched(this.moderatorIDs).length,
        });

      /*
         to support multiple items in a purchase in the future, pass in listings in the options,
         and add them to the order as items here.
      */
      this.listings.forEach(listing => {
        const item = new Item(
          {
            listingHash: listing.cid,
            quantity: listing.listing.metadata.contractType !== 'CRYPTOCURRENCY' ? bigNumber('1') : undefined,
            options: listing.options || [],
          },
          {
            isCrypto: listing.listing.metadata.contractType === 'CRYPTOCURRENCY',
            // inventory: () =>
            //   (
            //     typeof this.inventory === 'number' ?
            //       this.inventory : 99999999999999999
            //   ),
            getCoinDiv: () => this.coinDivisibility,
            getCoinType: () => listing.listing.metadata.pricingCurrency.code,
          }
        );
        // add the item to the order.
        this.order.get('items').add(item);
      })
      _.extend(this, this.order.toJSON());


      this.actionBtn = this.createChild(ActionBtn, {
        listing: this.listing,
      });
      this.listenTo(this.actionBtn, 'purchase', () => this.purchaseListing());
      this.listenTo(this.actionBtn, 'close', () => this.close());
      this.listenTo(this.actionBtn, 'reloadOutdated', () => {
        let defaultPrevented = false;

        this.trigger('clickReloadOutdated', {
          preventDefault: () => (defaultPrevented = true),
        });

        setTimeout(() => {
          if (!defaultPrevented) {
            Backbone.history.loadUrl();
          }
        });
      });

      this.coupons = this.createChild(Coupons, {
        coupons: this.listing.get('coupons'),
        listingPrice: bigNumber(this.listing.price.amount),
      });
      this.listenTo(this.coupons, 'changeCoupons', (hashes, codes) => this.changeCoupons(hashes, codes));

      const currencies = this.listing.get('metadata').get('acceptedCurrencies') || [];
      const locale = app.localSettings.standardizedTranslatedLang() || 'en-US';
      currencies.sort((a, b) => {
        const aName = app.polyglot.t(`cryptoCurrencies.${a}`, { _: a });
        const bName = app.polyglot.t(`cryptoCurrencies.${b}`, { _: b });
        return aName.localeCompare(bName, locale, { sensitivity: 'base' });
      });

      const disabledCurs = currencies.filter((c) => !isSupportedWalletCur(c));
      const activeCurs = currencies.length && this.listing.isCrypto ? [currencies[0]] : [];

      this.cryptoCurSelector = this.createChild(CryptoCurSelector, {
        disabledMsg: app.polyglot.t('purchase.cryptoCurrencyInvalid'),
        initialState: {
          controlType: 'radio',
          currencies,
          disabledCurs,
          sort: false,
          activeCurs,
        },
      });

      this.listenTo(this.cryptoCurSelector, 'currencyClicked', (cOpts) => {
        if (cOpts.active) this.moderators.setState({ showOnlyCur: cOpts.currency });
      });

      this.moderators = this.createChild(Moderators, {
        moderatorIDs: this.moderatorIDs,
        useCache: false,
        fetchErrorTitle: app.polyglot.t('purchase.errors.moderatorsTitle'),
        fetchErrorMsg: app.polyglot.t('purchase.errors.moderatorsMsg'),
        purchase: true,
        cardState: 'unselected',
        notSelected: 'unselected',
        singleSelect: true,
        radioStyle: true,
        initialState: {
          showOnlyCur: currencies[0],
          showVerifiedOnly: true,
        },
      });
      // render the moderators so it can start fetching and adding moderator cards
      this.moderators.render();
      this.moderators.getModeratorsByID();
      this.listenTo(this.moderators, 'noModsShown', () => this.render());
      this.listenTo(this.moderators, 'clickShowUnverified', () => {
        showVerifiedOnly = false;
      });
      this.listenTo(this.moderators, 'cardSelect', () => this.onCardSelect());

      if (this.listing.get('shippingOptions').length) {
        // set the initial shipping option
        this.updateShippingOption();
      }

      this.complete = this.createChild(Complete, {
        listing: this.listing,
        vendor: this.vendor,
      });

      // If the parent has the inventory, pass it in, otherwise we'll fetch it.
      // -- commenting out for now since inventory is not functioning properly on the server
      // this.inventory = this.options.inventory;
      // if (
      //   this.listing.isCrypto &&
      //   typeof this.inventory !== 'number'
      // ) {
      //   this.inventoryFetch = getInventory(
      //     this.listing.get('vendorID').peerID,
      //     {
      //       slug: this.listing.get('slug'),
      //       coinDivisibility:
      //         this.listing.get('metadata')
      //           .get('coinDivisibility'),
      //     }
      //   ).done(e => (this.inventory = e.inventory));
      //   this.listenTo(inventoryEvents, 'inventory-change',
      //     e => (this.inventory = e.inventory));
      // }

      this.listenTo(app.settings, 'change:localCurrency', () => this.showDataChangedMessage());
      this.listenTo(app.localSettings, 'change:bitcoinUnit', () => this.showDataChangedMessage());

      this.hasVerifiedMods = app.verifiedMods.matched(this.moderatorIDs).length > 0;

      this.listenTo(app.verifiedMods, 'update', () => {
        const newHasVerifiedMods = app.verifiedMods.matched(moderatorIDs).length > 0;
        if (newHasVerifiedMods !== this.hasVerifiedMods) {
          this.hasVerifiedMods = newHasVerifiedMods;
          this.showDataChangedMessage();
        }
      });

      this._latestHash = this.listing.get('hash');
      this._renderedHash = null;

      this.listenTo(outdatedListingHashesEvents, 'newHash', (e) => {
        this._latestHash = e.oldHash;
        if (e.oldHash === this._renderedHash) this.outdateHash();
      });

      this.render();
    },

    showDataChangedMessage () {
      if (this.dataChangePopIn && !this.dataChangePopIn.isRemoved()) {
        this.dataChangePopIn.$el.velocity('callout.shake', { duration: 500 });
      } else {
        this.dataChangePopIn = this.createChild(PopInMessage, {
          messageText:
            buildRefreshAlertMessage(app.polyglot.t('purchase.purchaseDataChangedPopin')),
        });

        this.listenTo(this.dataChangePopIn, 'clickRefresh', () => {
          this.render();
          this.moderators.render();
        });

        this.listenTo(this.dataChangePopIn, 'clickDismiss', () => {
          this.dataChangePopIn.remove();
          this.dataChangePopIn = null;
        });

        this.getCachedEl('.js-popInMessages').append(this.dataChangePopIn.render().el);
      }
    },

    goToListing () {
      app.router.navigate(`${this.vendor.peerID}/store/${this.listing.get('slug')}`,
        { trigger: true });
      this.close();
    },

    clickGoToListing () {
      this.goToListing();
    },

    clickClose () {
      this.trigger('closeBtnPressed');
      this.close();
    },

    close () {
      app.router.closeVueModal();
    },

    handleDirectPurchaseClick () {
      if (!this.isModerated) return;

      this.moderators.deselectOthers();
      this.render(); // always render even if the state didn't change
    },

    togVerifiedModerators (bool) {
      this.moderators.togVerifiedShown(bool);
      this.showVerifiedOnly = bool;
    },

    onClickVerifiedOnly (e) {
      this.togVerifiedModerators($(e.target).prop('checked'));
    },

    onCardSelect () {
      this.render(); // always render even if the state didn't change
    },

    changeCryptoAddress (e) {
      this.order.get('items')
        .at(0)
        .set('paymentAddress', e.target.value);
    },

    setModelQuantity (quantity) {
      let cur = this.cryptoAmountCurrency

      if (this.listing.isCrypto && (typeof cur !== 'string' || !cur)) {
        throw new Error('Please provide the currency code as a valid, non-empty string.');
      }

      this.order.get('items')
        .at(0)
        .set({ quantity });
    },

    changeCryptoAmountCurrency (e) {
      this._cryptoAmountCurrency = e.target.value;
      const { quantity } = this.getFormData(
        this.getCachedEl('#cryptoAmount'),
      );
      this.setModelQuantity(quantity);
    },

    keyupQuantity (e) {
      // wait until they stop typing
      if (this.quantityKeyUpTimer) {
        clearTimeout(this.quantityKeyUpTimer);
      }

      this.quantityKeyUpTimer = setTimeout(() => {
        const { quantity } = this.getFormData($(e.target));
        if (this.listing.isCrypto) this._cryptoQuantity = quantity;
        this.setModelQuantity(quantity);
      }, 150);
    },

    clickNewAddress () {
      launchSettingsModal({ initialTab: 'Addresses' });
    },

    applyCoupon () {
      this.coupons
        .addCode(this.couponField.val())
        .then((result) => {
          // if the result is valid, clear the input field
          if (result.type === 'valid') {
            this.couponField.val('');
          }
        });
    },

    onKeyUpCouponCode (e) {
      if (e.which === 13) {
        this.applyCoupon();
      }
    },

    blurEmailAddress (e) {
      this.order.set('alternateContactInfo', $(e.target).val());
    },

    blurMemo (e) {
      this.order.get('items').at(0).set('memo', $(e.target).val());
    },

    changeCoupons (hashes, codes) {
      // combine the codes and hashes so the receipt can check both.
      // if this is the user's own listing they will have codes instead of hashes
      const hashesAndCodes = hashes.concat(codes);
      const filteredCoupons = this.listing.get('coupons').filter(
        (coupon) => hashesAndCodes.indexOf(coupon.get('hash') || coupon.get('discountCode')) !== -1,
      );
      this.couponObj = filteredCoupons.map((coupon) => coupon.toJSON());

      this.order.get('items').at(0).set('coupons', codes);
    },

    updateShippingOption (selectedOption) {
      // Set the shipping option.
      this.shipping.selectedAddress = selectedOption;

      // this.order.get('items').at(0).get('shipping').set(selectedOption);
    },

    outdateHash () {
      this.actionBtn.setState({ outdatedHash: true });
    },

    purchaseListing () {
      // Clear any old errors.
      const allErrContainers = $('div[class $="-errors"]');
      allErrContainers.each((i, container) => $(container).html(''));

      // Don't allow a zero or negative price purchase.
      const priceObj = this.prices[0];
      if (
        priceObj
          .price
          .plus(priceObj.vPrice)
          .plus(priceObj.sPrice).lte(0)
      ) {
        this.insertErrors(this.getCachedEl('.js-errors'),
          [app.polyglot.t('purchase.errors.zeroPrice')]);
        this.phase = 'pay';
        return;
      }

      // Set the payment coin.
      let paymentCoin = this.paymentCoin;
      this.order.set({ paymentCoin });

      // Set the shipping address if the listing is shippable.
      if (this.shipping && this.shipping.selectedAddress) {
        this.order.addAddress(this.shipping.selectedAddress);
      }

      // Set the moderator.
      const moderator = this.moderators.selectedIDs[0] || '';
      this.order.set({ moderator });
      this.order.set({}, { validate: true });

      // Cancel any existing order.
      if (this.orderSubmit) this.orderSubmit.abort();

      this.phase = 'processing';

      startAjaxEvent('Purchase');
      const segmentation = {
        paymentCoin,
        moderated: !!moderator,
      };

      if (!this.order.validationError) {
        if (this.listing.isOwnListing) {
          this.phase = 'pay';
          // don't allow a seller to buy their own items
          const errTitle = app.polyglot.t('purchase.errors.ownIDTitle');
          const errMsg = app.polyglot.t('purchase.errors.ownIDMsg');
          openSimpleMessage(errTitle, errMsg);
          endAjaxEvent('Purchase', {
            ...segmentation,
            errors: 'own listing',
          });
        } else {
          const { coinDivisibility } = this;
          const cryptoItems = [];

          if (this.listing.isCrypto) {
            if (!isValidCoinDivisibility(coinDivisibility)[0]) {
              this.phase = 'pay';
              openSimpleMessage(
                app.polyglot.t('purchase.errors.genericPurchaseErrTitle'),
                app.polyglot.t('purchase.errors.invalidCoinDiv')
              );
              return;
            }

            try {
              const items = this.order.get('items');
              for (let i = 0; i < items.length; i += 2) {
                const item = items.at(i);
                cryptoItems.push({
                  ...item.toJSON(),
                  quantity: decimalToInteger(
                    item.get('quantity'),
                    coinDivisibility,
                  ),
                });
              }
            } catch (e) {
              this.phase = 'pay';
              openSimpleMessage(
                app.polyglot.t('purchase.errors.genericPurchaseErrTitle'),
                app.polyglot.t('purchase.errors.unableToConvertCryptoQuantity')
              );
              console.error(e);
              return;
            }
          }

          // Strip the 'cid' so it doesn't go to the server. Normally this is
          // done in the sync of the baseModel, but since we're POSTing outside of
          // that, we'll replicate that cleanup here.
          const postData = removeProp(
            {
              ...this.order.toJSON(),
              items: this.listing.isCrypto
                ? cryptoItems : this.order.get('items').toJSON(),
            },
            'cid',
          );

          $.post({
            url: app.getServerUrl('ob/purchase'),
            data: JSON.stringify(postData),
            dataType: 'json',
            contentType: 'application/json',
          })
            .done((data) => {
              this.phase = 'pending';
              this.payment = this.createChild(Payment, {
                balanceRemaining: curDefToDecimal(data.amount),
                paymentAddress: data.paymentAddress,
                orderID: data.orderID,
                isModerated: !!this.order.get('moderator'),
                metricsOrigin: 'Purchase',
                paymentCoin,
              });
              this.listenTo(this.payment, 'walletPaymentComplete', ((pmtCompleteData) => this.completePurchase(pmtCompleteData)));
              $('.js-pending').append(this.payment.render().el);
              endAjaxEvent('Purchase');
            })
            .fail((jqXHR) => {
              this.phase = 'pay';
              if (jqXHR.statusText === 'abort') return;
              let errTitle = app.polyglot.t('purchase.errors.orderError');
              let errMsg = (jqXHR.responseJSON && jqXHR.responseJSON.reason) || '';

              if (jqXHR.responseJSON
                && jqXHR.responseJSON.code === 'ERR_INSUFFICIENT_INVENTORY'
                && typeof jqXHR.responseJSON.remainingInventory === 'number') {
                this.inventory = jqXHR.responseJSON.remainingInventory
                  / this.coinDivisibility;
                errTitle = app.polyglot.t('purchase.errors.insufficientInventoryTitle');
                errMsg = app.polyglot.t('purchase.errors.insufficientInventoryBody', {
                  smart_count: this.inventory,
                  remainingInventory: new Intl.NumberFormat(app.settings.get('localCurrency'), {
                    minimumFractionDigits: 0,
                    maximumFractionDigits: 8,
                  }).format(this.inventory),
                });
                if (this.inventoryFetch) this.inventoryFetch.abort();
              } else if (errMsg === ERROR_DUST_AMOUNT) {
                errMsg = app.polyglot.t('purchase.errors.serverErrorBelowDust');
              }

              openSimpleMessage(errTitle, errMsg);
              endAjaxEvent('Purchase', {
                ...segmentation,
                errors: errMsg || 'unknown error',
              });
            });
        }
      } else {
        this.phase = 'pay';
        const purchaseErrs = {};
        Object.keys(this.order.validationError).forEach((errKey) => {
          const domKey = errKey.replace(/\[[^\[\]]*\]/g, '').replace('.', '-');
          let container = $(`.js-${domKey}-errors`);
          // if no container exists, use the generic container
          container = container.length ? container : this.getCachedEl('.js-errors');
          const err = this.order.validationError[errKey];
          this.insertErrors(container, err);
          purchaseErrs[`UserError-${domKey}`] = err.join(', ');
        });
        endAjaxEvent('Purchase', {
          ...segmentation,
          errors: 'User Error',
          ...purchaseErrs,
        });
      }
    },

    insertErrors (container, errors = []) {
      loadTemplate('formError.html', (t) => {
        container.html(t({
          errors,
        }));
      });
    },

    completePurchase (data) {
      this.complete.orderID = data.orderID;
      this.complete.render();
      this.phase = 'complete';
    },

    remove () {
      if (this.orderSubmit) this.orderSubmit.abort();
      if (this.inventoryFetch) this.inventoryFetch.abort();
      clearTimeout(this.quantityKeyUpTimer);
    },

    render () {
      if (this.dataChangePopIn) this.dataChangePopIn.remove();
      const item = this.order.get('items')
        .at(0);
      const quantity = item.get('quantity');
      const metadata = this.listing.get('metadata');

      let uiQuantity = quantity;

      if (this.listing.isCrypto && this._cryptoQuantity !== undefined) {
        uiQuantity = uiQuantity instanceof bigNumber && !uiQuantity.isNaN()
          ? toStandardNotation(this._cryptoQuantity) : this._cryptoQuantity;
      }


      this._$couponField = null;

      this.actionBtn.delegateEvents();
      this.actionBtn.setState({ phase: this.phase }, { renderOnChange: false });
      $('.js-actionBtn').append(this.actionBtn.render().el);

      this.coupons.delegateEvents();
      $('.js-couponsWrapper').html(this.coupons.render().el);

      this.moderators.delegateEvents();
      $('.js-moderatorsWrapper').append(this.moderators.el);

      this.cryptoCurSelector.delegateEvents();
      $('.js-cryptoCurSelectorWrapper').append(this.cryptoCurSelector.render().el);

      // if this is a re-render, and the payment exists, render it
      if (this.payment) {
        this.payment.delegateEvents();
        $('.js-pending').append(this.payment.render().el);
      }

      this.complete.delegateEvents();
      $('.js-complete').append(this.complete.render().el);

      if (this.feeChange) this.feeChange.remove();
      this.feeChange = this.createChild(FeeChange);
      $('.js-feeChangeContainer').html(this.feeChange.render().el);

      if (this.listing.isCrypto) {
        if (this.cryptoTitle) this.cryptoTitle.remove();
        this.cryptoTitle = this.createChild(CryptoTradingPair, {
          initialState: {
            tradingPairClass: 'cryptoTradingPairXL',
            exchangeRateClass: 'clrT2 tx6',
            fromCur: metadata.get('acceptedCurrencies')[0],
            toCur: this.listing.get('item').get('cryptoListingCurrencyCode'),
          },
        });
        this.getCachedEl('.js-cryptoTitle')
          .html(this.cryptoTitle.render().el);

        $('#cryptoAmountCurrency').select2({ minimumResultsForSearch: Infinity });
      }

      this._renderedHash = this.listing.get('hash');

      return this;
    }
  }
}
</script>
<style lang="scss" scoped>
</style>
