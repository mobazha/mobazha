<template>
  <div class="modal purchase modalScrollPage">
    <BaseModal :modalInfo="{ showCloseButton: false }">
      <template v-slot:component>
        <div ref="popInMessages" class="popInMessageHolder js-popInMessages"></div>

        <div class="topControls gutterHSm flex">
          <template v-if="ob.vendor">
            <div class="contentBox clrP clrSh3 clrBr clrT">
              <div class="padSm gutterHSm overflowAuto margRSm flexVCent">
                <a class="clrBr2 clrSh1 discTn flexNoShrink" :style="ob.getAvatarBgImage(ob.vendor.avatarHashes)"></a>
                <p class="txUnl tx3 clamp">{{ ob.vendor.name }}</p>
                <a class="link flexNoShrink tx6" @click="clickGoToListing">{{ ob.polyT('purchase.returnToListing') }}</a>
              </div>
            </div>
          </template>
        </div>

        <div :class="`flexRow gutterH mainSection ${ob.phaseClass}`">
          <div class="col9">
            <div class="flexColRow gutterV">
              <template v-for="(listing, key) in ob.listings" :key="key">
                <section class="contentBox pad clrP clrBr clrSh3">
                  <div class="js-errors">
                    <FormError v-if="errors['js-errors']" :errors="errors['js-errors']" />
                  </div>
                  <div class="js-items-quantity-errors">
                    <FormError v-if="errors['items-quantity']" :errors="errors['items-quantity']" />
                  </div>
                  <template v-if="!ob.isCrypto">
                    <div class="flexVCent gutterH">
                      <div class="thumb" :style="ob.getListingBgImage(listing.item.images[0])"></div>
                      <div class="flexExpand">
                        <div class="flexCol gutterVTn">
                          <div class="width100 noOverflow">
                            <b>{{ listing.item.title }}</b>
                          </div>
                          <template v-for="(variant, j) in listing.options" :key="j">
                            <div class="width100 noOverflow">
                              <span class="clrT2">{{ variant.name }}: {{ variant.value }}</span>
                            </div>
                          </template>
                        </div>
                      </div>
                      <template v-if="ob.phase === 'pay' || ob.phase === 'processing'">
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
                                type="number"
                                id="purchaseQuantity"
                                size="3"
                                name="quantity"
                                v-model="listing.quantity"
                                @keyup="keyupQuantity"
                                placeholder="0"
                                data-var-type="bignumber">
                            </div>
                          </div>
                        </div>
                      </template>
                      <div class="pad flexNoShrink"><b>{{ listing.price }}</b></div>
                    </div>
                  </template>

                  <template v-else>
                    <div class="flexVCent gutterHLg row cryptoTitleWrap">
                      <div ref="cryptoTitle" :class="`js-cryptoTitle ${ob.phase !== 'pay' && ob.phase !== 'processing' ? 'flexExpand' : ''}`"></div>
                      <template v-if="ob.phase === 'pay' || ob.phase === 'processing'">
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
                                :value="ob.uiQuantity"
                                @keyup="keyupQuantity"
                                placeholder="0.0000"
                                size="8"
                                data-var-type="bignumber">
                              <template v-if="ob.displayCurrency !== listing.item.cryptoListingCurrencyCode">
                                <select
                                  id="cryptoAmountCurrency"
                                  @change="changeCryptoAmountCurrency"
                                  class="clrBr clrP nestInputRight">
                                  <option
                                    v-for="(cur, j) in [listing.item.cryptoListingCurrencyCode, ob.displayCurrency]"
                                    :key="j"
                                    :value="cur"
                                    :selected="cur === ob.cryptoAmountCurrency">{{ cur }}</option>
                                </select>
                              </template>
                            </div>
                          </div>
                        </div>
                      </template>
                      <div class="pad flexNoShrink">
                        <CryptoPrice :options="{
                          priceAmount: totalPrice,
                          priceCurrencyCode: pricingCurrency,
                          displayCurrency: ob.displayCurrency,
                          priceModifier: listing.item.cryptoListingPriceModifier,
                        }" />
                      </div>
                    </div>
                    <hr class="clrBr rowLg" />
                    <div class="rowSm">
                      <label class="h4 flexExpand required" for="purchaseCryptoAddress">{{ heading }}</label>
                    </div>
                    <div class="js-items-paymentAddress-errors">
                      <FormError v-if="errors['items-paymentAddress']" :errors="errors['items-paymentAddress']" />
                    </div>
                    <template v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                      <input type="text"
                        id="purchaseCryptoAddress"
                        @change="changeCryptoAddress"
                        :value="ob.items[0].paymentAddress"
                        :placeholder="ob.polyT('purchase.cryptoAddressPlaceholder', { coinType: coinName})"
                        class="clrBr clrP rowSm"
                        :maxlength="ob.itemConstraints.maxPaymentAddressLength" />
                    </template>

                    <template v-else>
                      <p class="cryptoPaymentAddress">{{ ob.items[0].paymentAddress }}</p>
                    </template>
                    <div class="txSm clrT2">{{ helper }}</div>
                  </template>
                </section>
              </template>
            </div>
            <template v-if="ob.phase === 'pay' || ob.phase === 'processing'">
              <template v-if="ob.listing.shippingOptions && ob.listing.shippingOptions.length">
                <section class="contentBox padMd clrP clrBr clrSh3 js-shipping">
                  <div class="js-shipping-errors js-items-shipping-errors">
                    <FormError v-if="errors['shipping']" :errors="errors['shipping']" />
                    <FormError v-if="errors['items-shipping']" :errors="errors['items-shipping']" />
                  </div>
                  <Shipping
                    v-if="listing.get('shippingOptions').length"
                    :bb="function() {
                      return {
                        model: listing,
                      };
                    }"
                    @shippingOptionSelected="updateShippingOption"
                    @newAddress="clickNewAddress"
                    />
                </section>
              </template>
              <section class="contentBox padMd clrP clrBr clrSh3">
                <div class="flexColRows gutterVSm">
                  <div>
                    <div class="js-paymentCoin-errors">
                      <FormError v-if="errors['paymentCoin']" :errors="errors['paymentCoin']" />
                    </div>
                    <h2 class="h4 flexExpand required">{{ ob.polyT('purchase.cryptoCurrencyTitle') }}</h2>
                    <CryptoCurSelector
                      ref="cryptoCurSelector"
                      :options="{
                        disabledMsg: ob.polyT('purchase.cryptoCurrencyInvalid'),
                        initialState: {
                          controlType: 'radio',
                          currencies,
                          disabledCurs: currencies.filter((c) => !isSupportedWalletCur(c)),
                          sort: false,
                          activeCurs: currencies.length && listing.isCrypto ? [currencies[0]] : [],
                        },
                      }"
                      @currencyClicked="(cOpts) => {
                        if (cOpts.active) this.moderators.setState({ showOnlyCur: cOpts.currency });
                      }"/>
                  </div>
                </div>
              </section>
              <section class="contentBox padMd clrP clrBr clrSh3">
                <div class="flexColRows gutterVSm">
                  <div class="flexVCentClearMarg">
                    <h2 class="h4 flexExpand required">{{ ob.polyT('purchase.paymentTypeTitle') }}</h2>
                    <template v-if="ob.showModerators">
                      <input type="checkbox" id="purchaseVerifiedOnly" @click="onClickVerifiedOnly" :checked="ob.showVerifiedOnly">
                      <label class="tx5b" for="purchaseVerifiedOnly">{{ ob.polyT('settings.storeTab.verifiedOnly') }}</label>
                    </template>
                  </div>
                  <template v-if="ob.showModerators && !ob.noValidModerators">
                    <div class="js-moderated-errors">
                      <FormError v-if="errors['moderated']" :errors="errors['moderated']" />
                    </div>
                    <div ref="moderatorsWrapper" class="js-moderatorsWrapper"></div>
                    <template v-if="!ob.noValidModerators">
                      <div>
                        <div class="clrT2 tx6 rowMd">{{ ob.polyT('purchase.moderatorsDisclaimer') }}</div>
                      </div>
                    </template>
                    <hr class="clrBr row">
                  </template>
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
                        :value="ob.alternateContactInfo">
                    </div>
                    <div>
                      <span class="txSm clrT2">{{ ob.polyT('purchase.emailNote') }}</span>
                    </div>
                  </div>
                  <div class="col6">
                    <template v-if="ob.hasCoupons">
                      <div class="rowTn">
                        <label for="couponCode" class="tx5">{{ ob.polyT('purchase.couponCode') }}</label>
                      </div>
                      <div class="flex gutterH row">
                        <input
                          class="btnHeight clrBr clrP"
                          type="text"
                          id="couponCode"
                          @keyup.enter="applyCoupon"
                          v-model="couponCode"
                          name="couponCode"
                          :placeholder="ob.polyT('purchase.couponCodePlaceholder')">
                        <button class="btn clrP clrBr clrSh2 flexNoShrink" @click="applyCoupon">
                          {{ ob.polyT('purchase.applyCode') }}
                        </button>
                      </div>
                      <div class="js-couponsWrapper">
                        <Coupons :options="{
                            coupons: _listing.coupons,
                            listingPrice: ob.bigNumber(listing.price.amount),
                          }"
                          @changeCoupons="changeCoupons"/>
                        <!-- // coupons are inserted here after they are added by the user. -->
                      </div>
                    </template>
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
                  :placeholder="ob.polyT('purchase.memoPlaceholder')"
                  v-model="ob.items[0].memo"></textarea>
              </section>
            </template>
            <template v-if="ob.phase === 'pending'">
              <section ref="pendingPayment" class="contentBox padMd clrP clrBr clrSh3 js-pending"></section>
            </template>
            <template v-if="ob.phase === 'complete'">
              <section class="contentBox padMd clrP clrBr clrSh3 js-complete">
                <Complete :options="{
                    vendor,
                    orderID,
                  }"
                  :bb="function() {
                    return {
                      listing,
                    };
                  }"
                />
              </section>
            </template>
          </div>
          <div class="col3">
            <section class="contentBox pad clrP clrBr clrSh3 sidebar">
              <i class="cornerTR ion-ios-close-empty iconBtn clrP clrBr clrSh3 closeBtn" @click="clickClose"></i>
              <div class="js-actionBtn">
                <ActionBtn
                  ref="actionBtn"
                  :options="{
                    initialState: {
                      phase: ob.phase,
                      outdatedHash,
                    },
                  }"
                  :bb="function() {
                    return {
                      listing,
                    };
                  }"
                  @purchase="purchaseListing"
                  @close="close"
                  @reloadOutdated="onReloadOutdated"
                />
              </div>
              <div class="rowLg">
                <!-- <div class="js-receipt"></div> -->
                <Receipt
                  v-if="order"
                  :options="{
                    prices: prices,
                    coupons: couponObj,
                    showTotalTip: _state.phase === 'pay',
                  }"
                  :bb="function() {
                    return {
                      model: order,
                      listing,
                    };
                  }"
                />
                <template v-if="ob.showModerators && !ob.noValidModerators">
                  <hr class="clrBr">
                  <div class="padSm txSm txCtr clrT2">
                    {{ ob.polyT('purchase.moderatorNote') }}
                  </div>
                </template>
              </div>
              <div ref="feeChangeContainer" class="tx6 js-feeChangeContainer"></div>
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

import Payment from '../../../../backbone/views/modals/purchase/Payment';

import ActionBtn from './ActionBtn.vue';
import Complete from './Complete.vue';
import Coupons from './Coupons.vue';
import DirectPayment from './DirectPayment.vue';
import Receipt from './Receipt.vue';
import Shipping from './Shipping.vue';

import { toRaw } from 'vue';

export default {
  components: {
    ActionBtn,
    Complete,
    Coupons,
    Receipt,
    DirectPayment,
    Shipping,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      _state: {
        phase: 'pay',
      },

      cart: {},
      vendor: {},
      order: undefined,
      items: [],

      listings: undefined,
      moderators: undefined,
      couponObj: [],

      coinName: '',
      moderatorIDs: [],
      showVerifiedOnly: true,
      shipping: {
        selectedAddress: '',
      },

      outdatedHash: false,

      orderID: '',
      couponCode: '',

      errors: {},
    };
  },
  created () {
    this.initEventChain();

    // for shopping cart
    // this.init();
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      const item = this.order.get('items').at(0);
      let uiQuantity = item ? item.get('quantity') : 0;

      if (this.listing?.isCrypto && this._cryptoQuantity !== undefined) {
        uiQuantity = uiQuantity instanceof bigNumber && !uiQuantity.isNaN()
          ? toStandardNotation(this._cryptoQuantity) : this._cryptoQuantity;
      }

      return {
        ...this.templateHelpers,
        ...this.order.toJSON(),
        ...this._state,
        listing: this.listing.toJSON(),
        listings: this.listings.map((listing) => listing.toJSON()),
        listingPrice: this.listing.price,
        itemConstraints: this.order.get('items')
          .at(0)
          .constraints,
        vendor: this.vendor,
        variants: this.variants,
        prices: this.prices,
        displayCurrency: app.settings.get('localCurrency'),
        quantity: uiQuantity,
        cryptoAmountCurrency: this.cryptoAmountCurrency,
        isCrypto: this.listing.isCrypto,
        phaseClass: `phase${capitalize(this._state.phase)}`,
        hasCoupons: this.listing.get('coupons').length
          && this.listing.get('metadata').get('contractType') !== 'CRYPTOCURRENCY',
      }
    },
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
      return this.$refs.cryptoCurSelector ? this.$refs.cryptoCurSelector.getState().activeCurs[0] : '';
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
          aPrice: bigNumber(sOptService ? sOptService.get('additionalWeightPrice') || 0 : 0),
          vPrice: bigNumber(sku ? sku.get('surcharge') || 0 : 0),
          quantity: bigNumber(item.get('quantity')),
        };
      });
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
    },

    currencies () {
      let currencies = this.listing.get('metadata').get('acceptedCurrencies') || [];
      const locale = app.localSettings.standardizedTranslatedLang() || 'en-US';
      currencies.sort((a, b) => {
        const aName = app.polyglot.t(`cryptoCurrencies.${a}`, { _: a });
        const bName = app.polyglot.t(`cryptoCurrencies.${b}`, { _: b });
        return aName.localeCompare(bName, locale, { sensitivity: 'base' });
      });

      return currencies;
    },
  },
  methods: {
    isSupportedWalletCur,

    init() {
      let options = {};
      let cart = this.$store.state.cart.cart;

      options.vendor = {
        peerID: cart.vendorID,
        name: cart.profile.name,
        handle: cart.profile.handle,
        avatarHashes: cart.profile.avatarHashes,
      };
      
      options.listings = toRaw(cart.listings);
      options.listing = cart.listings[0];

      options.variants = cart.items[0].options;

      this.loadData(options);

      // const coinType = this.listing.listing.item.cryptoListingCurrencyCode;
      // const coinTranslationKey = `cryptoCurrencies.${coinType}`;
      // this.coinName = ob.polyT(coinTranslationKey) === coinTranslationKey ? coinType : ob.polyT(coinTranslationKey);
    },
    loadData (options = {}) {
      if (!this.listing || !(this.listing instanceof Listing)) {
        throw new Error('Please provide a listing model');
      }

      if (!options.vendor) {
        throw new Error('Please provide a vendor object');
      }

      const opts = {
        ...options,
        initialState: {
          phase: 'pay',
          ...options.initialState || {},
        },
      };

      this.baseInit(opts);

      // this.listing = opts.listing;
      // this.variants = opts.variants;
      // this.vendor = opts.vendor;
      const shippingOptions = this.listing.get('shippingOptions');
      const moderatorIDs = this.listing.get('moderators') || [];
      const disallowedIDs = [app.profile.id, this.listing.get('vendorID').peerID];
      this.moderatorIDs = _.without(moderatorIDs, ...disallowedIDs);

      this.setState({
        showModerators: this.moderatorIDs.length,
        showVerifiedOnly: true,
      }, { renderOnChange: false });

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
      this.listings = [ this.listing ];
      this.listings.forEach(listing => {
        const item = new Item(
          {
            listingHash: listing.get('hash'),
            quantity: listing.isCrypto ? bigNumber('1') : undefined,
            options: listing.options || [],
          },
          {
            isCrypto: listing.isCrypto,
            // inventory: () =>
            //   (
            //     typeof this.inventory === 'number' ?
            //       this.inventory : 99999999999999999
            //   ),
            getCoinDiv: () => this.coinDivisibility,
            getCoinType: () => listing.get('metadata').get('coinType'),
          }
        );
        // add the item to the order.
        this.order.get('items').add(item);
      })

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
          showOnlyCur: this.currencies[0],
          showVerifiedOnly: true,
        },
      });
      // render the moderators so it can start fetching and adding moderator cards
      this.moderators.render();
      this.moderators.getModeratorsByID();
      this.listenTo(this.moderators, 'noModsShown', () => this.render());
      this.listenTo(this.moderators, 'clickShowUnverified', () => {
        this.setState({ showVerifiedOnly: false });
      });
      this.listenTo(this.moderators, 'cardSelect', () => this.onCardSelect());


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
    },

    onReloadOutdated() {
      let defaultPrevented = false;

      this.$emit('clickReloadOutdated', {
        preventDefault: () => (defaultPrevented = true),
      });

      setTimeout(() => {
        if (!defaultPrevented) {
          Backbone.history.loadUrl();
        }
      });
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

        $(this.$refs.popInMessages).append(this.dataChangePopIn.render().el);
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
      this.$emit('closeBtnPressed');
      this.close();
    },

    close () {
      this.$emit('close');
    },

    handleDirectPurchaseClick (options) {
      // { active: true }
      if (!this.isModerated) return;

      this.moderators.deselectOthers();
      this.setState({ unverifedSelected: false }, { renderOnChange: false });
      this.render(); // always render even if the state didn't change
    },

    togVerifiedModerators (bool) {
      this.moderators.togVerifiedShown(bool);
      this.setState({ showVerifiedOnly: bool });
    },

    onClickVerifiedOnly (e) {
      this.togVerifiedModerators($(e.target).prop('checked'));
    },

    onCardSelect () {
      const selected = this.moderators.selectedIDs;
      const unverifedSelected = selected.length && !app.verifiedMods.matched(selected).length;
      this.setState({ unverifedSelected }, { renderOnChange: false });
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
        $('#cryptoAmount'),
      );
      this.setModelQuantity(quantity);
    },

    keyupQuantity (e) {
      // wait until they stop typing
      if (this.quantityKeyUpTimer) {
        clearTimeout(this.quantityKeyUpTimer);
      }

      this.quantityKeyUpTimer = setTimeout(() => {
        let { quantity } = this.getFormData($(e.target));
        if (!_.isEmpty(quantity)) {
          quantity = bigNumber(quantity);
        }
        if (this.listing.isCrypto) this._cryptoQuantity = quantity;
        this.setModelQuantity(quantity);
      }, 150);
    },

    clickNewAddress () {
      launchSettingsModal({ initialTab: 'Addresses' });
    },

    applyCoupon () {
      this.coupons
        .addCode(this.couponCode)
        .then((result) => {
          // if the result is valid, clear the input field
          if (result.type === 'valid') {
            this.couponCode = '';
          }
        });
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
      this.outdatedHash = true;
    },

    purchaseListing () {
      // Clear any old errors.
      this.errors = {};

      // Don't allow a zero or negative price purchase.
      const priceObj = this.prices[0];
      if (
        priceObj
          .price
          .plus(priceObj.vPrice)
          .plus(priceObj.sPrice).lte(0)
      ) {
        this.insertErrors('js-errors', [app.polyglot.t('purchase.errors.zeroPrice')]);
        this.setState({ phase: 'pay' });
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

      this.setState({ phase: 'processing' });

      startAjaxEvent('Purchase');
      const segmentation = {
        paymentCoin,
        moderated: !!moderator,
      };

      if (!this.order.validationError) {
        if (this.listing.isOwnListing) {
          this.setState({ phase: 'pay' });
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
              this.setState({ phase: 'pay' });
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
              this.setState({ phase: 'pay' });
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
              this.setState({ phase: 'pending' });
              this.payment = this.createChild(Payment, {
                balanceRemaining: curDefToDecimal(data.amount),
                paymentAddress: data.paymentAddress,
                orderID: data.orderID,
                isModerated: !!this.order.get('moderator'),
                metricsOrigin: 'Purchase',
                paymentCoin,
              });
              this.listenTo(this.payment, 'walletPaymentComplete', ((pmtCompleteData) => this.completePurchase(pmtCompleteData)));
              $(this.$refs.pendingPayment).append(this.payment.render().el);
              endAjaxEvent('Purchase');
            })
            .fail((jqXHR) => {
              this.setState({ phase: 'pay' });
              if (jqXHR.statusText === 'abort') return;
              let errTitle = app.polyglot.t('purchase.errors.orderError');
              let errMsg = (jqXHR.responseJSON && jqXHR.responseJSON.reason) || '';

              if (jqXHR.responseJSON
                && jqXHR.responseJSON.code === 'ERR_INSUFFICIENT_INVENTORY'
                && typeof jqXHR.responseJSON.remainingInventory === 'number') {
                this.inventory = jqXHR.responseJSON.remainingInventory
                  / coinDivisibility;
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
        this.setState({ phase: 'pay' });
        const purchaseErrs = {};
        Object.keys(this.order.validationError).forEach((errKey) => {
          const domKey = errKey.replace(/\[[^\[\]]*\]/g, '').replace('.', '-');
          let container = domKey;
          // if no container exists, use the generic container
          container = container.length ? container : 'js-errors';
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
      this.errors[container] = errors;
    },

    completePurchase (data) {
      this.orderID = data.orderID;

      this.setState({ phase: 'complete' });
    },

    remove () {
      if (this.orderSubmit) this.orderSubmit.abort();
      if (this.inventoryFetch) this.inventoryFetch.abort();
      clearTimeout(this.quantityKeyUpTimer);
    },

    render () {
      if (this.dataChangePopIn) this.dataChangePopIn.remove();
      const state = this.getState();
      const metadata = this.listing.get('metadata');

      this.moderators.delegateEvents();
      $(this.$refs.moderatorsWrapper).append(this.moderators.el);

      // if this is a re-render, and the payment exists, render it
      if (this.payment) {
        this.payment.delegateEvents();
        $(this.$refs.pendingPayment).append(this.payment.render().el);
      }

      if (this.feeChange) this.feeChange.remove();
      this.feeChange = this.createChild(FeeChange);
      $(this.$refs.feeChangeContainer).html(this.feeChange.render().el);

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
        $(this.$refs.cryptoTitle).html(this.cryptoTitle.render().el);

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
