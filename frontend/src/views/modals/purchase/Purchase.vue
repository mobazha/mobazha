<template>
  <div class="modal purchase modalScrollPage">
    <div class="popInMessageHolder js-popInMessages"></div>

    <div class="topControls gutterHSm flex">
      <div v-if="ob.vendor" class="contentBox clrP clrSh3 clrBr clrT">
        <div class="padSm gutterHSm overflowAuto margRSm flexVCent">
          <a class="clrBr2 clrSh1 discTn flexNoShrink" :style="ob.getAvatarBgImage(ob.vendor.avatarHashes)"></a>
          <p class="txUnl tx3 clamp">{{ ob.vendor.name }}</p>
          <a class="link flexNoShrink tx6" @click="clickGoToListing">{{ ob.polyT('purchase.returnToListing') }}</a>
        </div>
      </div>
    </div>

    <div :class="`flexRow gutterH mainSection ${ob.phaseClass}`">
      <div class="col9">
        <div class="flexColRow gutterV">
          <section class="contentBox pad clrP clrBr clrSh3" v-for="(listing, key) in ob.listings" :key="key">
            <div class="js-errors"></div>
            <div class="js-items-quantity-errors"></div>

            <div class="flexVCent gutterH" v-if="!ob.isCrypto">
              <div class="thumb" :style="ob.getListingBgImage(listing.listing.item.images[0])"></div>
              <div class="flexExpand">
                <div class="flexCol gutterVTn">
                  <div class="width100 noOverflow">
                    <b>{{ listing.listing.item.title }}</b>
                  </div>

                  <div class="width100 noOverflow" v-for="(variant, i) in listing.options" :key="i">
                    <span class="clrT2">{{ variant.name }}: {{ variant.value }}</span>
                  </div>

                </div>
              </div>

              <div class="flexNoShrink" v-if="ob.phase === 'pay' || ob.phase === 'processing'">
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
                      :on-keyup="keyupQuantity"
                      placeholder="0"
                      data-var-type="bignumber">
                  </div>
                </div>
              </div>

              <div class="pad flexNoShrink"><b>{{ listing.price }}</b></div>
            </div>
            <div v-else>
              <div class="flexVCent gutterHLg row cryptoTitleWrap">
                <div :class="`js-cryptoTitle ${ob.phase !== 'pay' && ob.phase !== 'processing' ? 'flexExpand' : ''}`"></div>

                <div class="flexExpand" v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                  <div class="flexVCent gutterHLg">
                    <label for="cryptoAmount" class="clrT txB required">{{ ob.polyT('purchase.cryptoAmount') }}</label>
                    <div class="inputSelect">
                      <input
                        type="text"
                        class="clrBr clrP clrSh2"
                        name="quantity"
                        id="cryptoAmount"
                        :value="ob.quantity"
                        :on-keyup="keyupQuantity"
                        :on-change="onChangeCryptoAmount"
                        placeholder="0.0000"
                        size="8"
                        data-var-type="bignumber">

                      <select id="cryptoAmountCurrency"
                        class="clrBr clrP nestInputRight"
                        :on-change="changeCryptoAmountCurrency"
                        v-if="ob.displayCurrency !== listing.listing.item.cryptoListingCurrencyCode">
                        <option v-for="(cur, j) in [listing.listing.item.cryptoListingCurrencyCode, ob.displayCurrency]" :key="j" :value="cur" :selected="cur === cryptoAmountCurrency">{{ cur }}</option>
                      </select>
                    </div>
                  </div>
                </div>

                <div class="pad flexNoShrink">
                  {{ 
                    ob.crypto.cryptoPrice({
                      priceAmount: totalPrice,
                      priceCurrencyCode: pricingCurrency,
                      displayCurrency: ob.displayCurrency,
                      priceModifier: listing.listing.item.cryptoListingPriceModifier,
                    })
                  }}
                </div>
              </div>
              <hr class="clrBr rowLg" />

              <div class="rowSm">
                <label class="h4 flexExpand required" for="purchaseCryptoAddress">{{ ob.polyT('purchase.cryptoAddressHeading', {coinType: coinName}) }}</label>
              </div>
              <div class="js-items-paymentAddress-errors"></div>
              <div v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                <input type="text"
                  id="purchaseCryptoAddress"
                  :value="ob.items[0].paymentAddress"
                  :on-change="changeCryptoAddress"
                  :placeholder="ob.polyT('purchase.cryptoAddressPlaceholder',
                  { coinType: coinName})" class="clrBr clrP rowSm"
                  :maxlength="ob.itemConstraints.maxPaymentAddressLength" />
              </div>
              <p v-else class="cryptoPaymentAddress">{{ ob.items[0].paymentAddress }}</p>

              <div class="txSm clrT2">{{ helper }}</div>
            </div>
          </section>

          <div v-if="ob.phase === 'pay' || ob.phase === 'processing'">
            <section v-if="ob.listing.shippingOptions && ob.listing.shippingOptions.length" class="contentBox padMd clrP clrBr clrSh3 js-shipping">
              <div class="js-shipping-errors js-items-shipping-errors"></div>
              <div class="js-shippingWrapper"></div>
            </section>

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
                    <input type="checkbox" id="purchaseVerifiedOnly" :on-click="onClickVerifiedOnly" :checked="showVerifiedOnly">
                    <label class="tx5b" for="purchaseVerifiedOnly">{{ ob.polyT('settings.storeTab.verifiedOnly') }}</label>
                  </div>
                </div>
                <div v-if="showModerators && !ob.noValidModerators">
                  <div class="js-moderated-errors"></div>
                  <div class="js-moderatorsWrapper"></div>
                  <div v-if="!ob.noValidModerators">
                    <div class="clrT2 tx6 rowMd">{{ ob.polyT('purchase.moderatorsDisclaimer') }}</div>
                  </div>
                  <hr class="clrBr row">
                </div>
                <div class="js-directPaymentWrapper moderatorsList"></div>
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
                      :on-blur="blurEmailAddress"
                      :placeholder="ob.polyT('purchase.emailPlaceholder')"
                      :value="ob.alternateContactInfo">
                  </div>
                  <div>
                    <span class="txSm clrT2">{{ ob.polyT('purchase.emailNote') }}</span>
                  </div>
                </div>
                <div class="col6">
                  <div v-if="ob.hasCoupons">
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
                        name="couponCode"
                        :on-keyup="onKeyUpCouponCode"
                        :placeholder="ob.polyT('purchase.couponCodePlaceholder')">
                      <button class="btn clrP clrBr clrSh2 flexNoShrink" :on-click="applyCoupon">
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
                :on-blur="blurMemo"
                maxlength="5000"
                rows="6"
                v-model="ob.items[0].memo"
                :placeholder="ob.polyT('purchase.memoPlaceholder')" />
            </section>
          </div>

          <section v-if="ob.phase === 'pending'" class="contentBox padMd clrP clrBr clrSh3 js-pending"></section>

          <section v-if="ob.phase === 'complete'" class="contentBox padMd clrP clrBr clrSh3 js-complete"></section>
        </div>
      </div>
      <div class="col3">
        <section class="contentBox pad clrP clrBr clrSh3 sidebar">
          <i class="js-close cornerTR ion-ios-close-empty iconBtn clrP clrBr clrSh3 closeBtn"></i>
          <div class="js-actionBtn"></div>
          <div class="rowLg">
            <div class="js-receipt"></div>
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
  </div>
</template>

<script setup>
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
import BaseModal from '../../../../backbone/views/modals/BaseModal';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import PopInMessage, { buildRefreshAlertMessage } from '../../../../backbone/views/components/PopInMessage';
import Moderators from '../../../../backbone/views/components/moderators/Moderators';
import FeeChange from '../../../../backbone/views/components/FeeChange';
import CryptoTradingPair from '../../../../backbone/views/components/CryptoTradingPair';
import CryptoCurSelector from '../../../../backbone/views/components/CryptoCurSelector';
import Shipping from '../../../../backbone/views/modals/purchase/Shipping';
import Receipt from '../../../../backbone/views/modals/purchase/Receipt';
import Coupons from '../../../../backbone/views/modals/purchase/Coupons';
import ActionBtn from '../../../../backbone/views/modals/purchase/ActionBtn';
import Payment from '../../../../backbone/views/modals/purchase/Payment';
import Complete from '../../../../backbone/views/modals/purchase/Complete';
import DirectPayment from '../../../../backbone/views/modals/purchase/DirectPayment';

import { reactive } from 'vue';

const props = defineProps({
  listings: Object,
  listing: Object,
  variants: Object,
  vendor: Object,
})

loadData(props);

let ob = {};
render();

let inventory = 0;

// when multiple listings are supported, the prices array will have one price object for each
const totalPrice =
  ob.prices[0]
    .price
    .plus(ob.prices[0].vPrice);

const pricingCurrency = ob.listingPrice.currencyCode;


const coinType = listing.listing.item.cryptoListingCurrencyCode;
const coinTranslationKey = `cryptoCurrencies.${coinType}`;
const coinName = ob.polyT(coinTranslationKey) === coinTranslationKey ? coinType : ob.polyT(coinTranslationKey);

const warning = this.phase === 'pay' || this.phase === 'processing' ?
  `<b>${ob.polyT('purchase.cryptoAddressHelperWarning')}</b>` :
  `<b>${ob.polyT('purchase.cryptoAddressHelperWarning2')}</b>`;
const helper = ob.polyT('purchase.cryptoAddressHelper', {
  name: ob.vendor.name,
  coinType: coinName,
  warning,
});

const showModerators = computed(() => {
  return this.moderatorIDs?.length;
});

let showVerifiedOnly = true;


const prices = computed(() => {
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
  });

  function refreshPrices() {
    // this.receipt.updatePrices(this.prices);
  }

  const couponField = computed(() => {
    if (!this._couponField) {
      this._couponField = this.$('#couponCode');
    }
    return this._couponField;
  })

  const cryptoAmountCurrency = computed(() => {
    return this._cryptoAmountCurrency
      || this.listing.get('item')
        .get('cryptoListingCurrencyCode');
  });

  const coinDivisibility = computed((listing) => {
    let currencyCode = listing.isCrypto ? listing.item?.cryptoListingCurrencyCode : listing.metadata?.pricingCurrency?.code;

    let coinDiv;
    try {
      coinDiv = getCoinDivisibility(currencyCode);
    } catch (e) {
      // pass
    }
    return coinDiv;
  });

  const isModerated = computed(() => {
    return this.moderators.selectedIDs.length > 0;
  });

  

  function loadData(options = {}) {
    this.phase = 'pay';

    this.cart = opts.cart;
    this.listings = [];
    this.cart.items.forEach(item => {
      console.log("item: ", item)
      this.listings.push(item)
    });

    this.listing = opts.listing;
    this.variants = opts.variants;
    this.vendor = opts.vendor;
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
          getCoinDiv: () => coinDivisibility(listing.listing),
          getCoinType: () => listing.listing.metadata.pricingCurrency.code,
        }
      );
      // add the item to the order.
      this.order.get('items').add(item);
    })
    

    this.actionBtn = this.createChild(ActionBtn, {
      listing: this.listing,
    });
    this.listenTo(this.actionBtn, 'purchase', () => purchaseListing());
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

    // this.receipt = this.createChild(Receipt, {
    //   model: this.order,
    //   listing: this.listing,
    //   prices: this.prices,
    //   couponObj: this.couponObj,
    //   showTotalTip: this.getState().phase === 'pay',
    // });

    this.coupons = this.createChild(Coupons, {
      coupons: this.listing.get('coupons'),
      listingPrice: bigNumber(this.listing.price.amount),
    });
    this.listenTo(this.coupons, 'changeCoupons', (hashes, codes) => changeCoupons(hashes, codes));

    const currencies = this.listings[0]?.metadata?.acceptedCurrencies || [];
    const locale = app.localSettings.standardizedTranslatedLang() || 'en-US';
    currencies.sort((a, b) => {
      const aName = app.polyglot.t(`cryptoCurrencies.${a}`, { _: a });
      const bName = app.polyglot.t(`cryptoCurrencies.${b}`, { _: b });
      return aName.localeCompare(bName, locale, { sensitivity: 'base' });
    });

    const disabledCurs = currencies.filter((c) => !isSupportedWalletCur(c));
    const activeCurs = currencies.length && this.listings[0]?.isCrypto ? [currencies[0]] : [];

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
      this.receipt.paymentCoin = cOpts.active ? cOpts.currency : '';
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
    this.listenTo(this.moderators, 'cardSelect', () => onCardSelect());

    if (this.listing[0]?.shippingOptions.length) {
      this.shipping = this.createChild(Shipping, {
        model: this.listing,
      });
      this.listenTo(this.shipping, 'shippingOptionSelected', () => updateShippingOption());
      // set the initial shipping option
      updateShippingOption();
      this.refreshPrices();
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

    this.listenTo(app.settings, 'change:localCurrency', () => showDataChangedMessage());
    this.listenTo(app.localSettings, 'change:bitcoinUnit', () => showDataChangedMessage());
    this.listenTo(this.order.get('items').at(0), 'someChange ', () => this.refreshPrices());
    this.listenTo(this.order.get('items').at(0).get('shipping'), 'change', () => this.refreshPrices());

    this.hasVerifiedMods = app.verifiedMods.matched(this.moderatorIDs).length > 0;

    this.listenTo(app.verifiedMods, 'update', () => {
      const newHasVerifiedMods = app.verifiedMods.matched(moderatorIDs).length > 0;
      if (newHasVerifiedMods !== this.hasVerifiedMods) {
        this.hasVerifiedMods = newHasVerifiedMods;
        showDataChangedMessage();
      }
    });

    this._latestHash = this.listing.get('hash');
    this._renderedHash = null;

    this.listenTo(outdatedListingHashesEvents, 'newHash', (e) => {
      this._latestHash = e.oldHash;
      if (e.oldHash === this._renderedHash) outdateHash();
    });
  }


  function events() {
    return {
      'click .js-close': 'clickClose',
      'click .js-newAddress': 'clickNewAddress',
      ...super.events(),
    };
  }

  function showDataChangedMessage() {
    if (this.dataChangePopIn && !this.dataChangePopIn.isRemoved()) {
      this.dataChangePopIn.$el.velocity('callout.shake', { duration: 500 });
    } else {
      this.dataChangePopIn = this.createChild(PopInMessage, {
        messageText:
          buildRefreshAlertMessage(app.polyglot.t('purchase.purchaseDataChangedPopin')),
      });

      this.listenTo(this.dataChangePopIn, 'clickRefresh', () => {
        this.moderators.render();
      });

      this.listenTo(this.dataChangePopIn, 'clickDismiss', () => {
        this.dataChangePopIn.remove();
        this.dataChangePopIn = null;
      });

      this.getCachedEl('.js-popInMessages').append(this.dataChangePopIn.render().el);
    }
  }

  function goToListing() {
    app.router.navigate(`${this.vendor.peerID}/store/${this.listing.get('slug')}`,
      { trigger: true });
    this.close();
  }

  function clickGoToListing() {
    // this.goToListing();
    const receipt = createApp(Receipt, {
      model: this.order,
      listing: this.listing.toJSON(),
      prices: this.prices,
      coupons: this.couponObj,
      showTotalTip: this.phase === 'pay',
    });
    receipt.mount('#jsReceipt');
  }

  function clickClose() {
    this.trigger('closeBtnPressed');
    this.close();
  }

  function handleDirectPurchaseClick() {
    if (!this.isModerated) return;

    this.moderators.deselectOthers();
  }

  function togVerifiedModerators(bool) {
    this.moderators.togVerifiedShown(bool);
    showVerifiedOnly = bool;
  }

  function onClickVerifiedOnly(e) {
    togVerifiedModerators($(e.target).prop('checked'));
  }

  function onCardSelect() {
    const selected = this.moderators.selectedIDs;
  }

  function changeCryptoAddress(e) {
    this.order.get('items')
      .at(0)
      .set('paymentAddress', e.target.value);
  }

  function setModelQuantity(quantity, cur = cryptoAmountCurrency) {
    if (this.listing.isCrypto && (typeof cur !== 'string' || !cur)) {
      throw new Error('Please provide the currency code as a valid, non-empty string.');
    }

    this.order.get('items')
      .at(0)
      .set({ quantity });
  }

  function changeCryptoAmountCurrency(e) {
    this._cryptoAmountCurrency = e.target.value;
    const { quantity } = this.getFormData(
      this.getCachedEl('#cryptoAmount'),
    );
    setModelQuantity(quantity);
  }

  function keyupQuantity(e) {
    // wait until they stop typing
    if (this.quantityKeyUpTimer) {
      clearTimeout(this.quantityKeyUpTimer);
    }

    this.quantityKeyUpTimer = setTimeout(() => {
      const { quantity } = this.getFormData($(e.target));
      if (this.listing.isCrypto) this._cryptoQuantity = quantity;
      setModelQuantity(quantity);
    }, 150);
  }

  function clickNewAddress() {
    launchSettingsModal({ initialTab: 'Addresses' });
  }

  function applyCoupon() {
    this.coupons
      .addCode(couponField.val())
      .then((result) => {
        // if the result is valid, clear the input field
        if (result.type === 'valid') {
          couponField.val('');
        }
      });
  }

  function onKeyUpCouponCode(e) {
    if (e.which === 13) {
      applyCoupon();
    }
  }

  function blurEmailAddress(e) {
    this.order.set('alternateContactInfo', $(e.target).val());
  }

  function blurMemo(e) {
    this.order.get('items').at(0).set('memo', $(e.target).val());
  }

  function changeCoupons(hashes, codes) {
    // combine the codes and hashes so the receipt can check both.
    // if this is the user's own listing they will have codes instead of hashes
    const hashesAndCodes = hashes.concat(codes);
    const filteredCoupons = this.listing.get('coupons').filter(
      (coupon) => hashesAndCodes.indexOf(coupon.get('hash') || coupon.get('discountCode')) !== -1,
    );
    this.couponObj = filteredCoupons.map((coupon) => coupon.toJSON());
    this.receipt.coupons = this.couponObj;
    this.order.get('items').at(0).set('coupons', codes);
  }

  function updateShippingOption() {
    // Set the shipping option.
    this.order.get('items').at(0).get('shipping')
      .set(this.shipping.selectedOption);
  }

  function outdateHash() {
    this.actionBtn.setState({ outdatedHash: true });
  }

  function purchaseListing() {
    // Clear any old errors.
    const allErrContainers = this.$('div[class $="-errors"]');
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
    const paymentCoin = this.cryptoCurSelector.getState().activeCurs[0];
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
            this.listenTo(this.payment, 'walletPaymentComplete', ((pmtCompleteData) => completePurchase(pmtCompleteData)));
            this.$('.js-pending').append(this.payment.render().el);
            endAjaxEvent('Purchase');
          })
          .fail((jqXHR) => {
            handlePurchaseResponseFailed(jqXHR);

            endAjaxEvent('Purchase', {
              ...segmentation,
              errors: errMsg || 'unknown error',
            });
          });
      }
    } else {
      this.phase = 'pay';
      const purchaseErrs = formatPurchaseError();

      endAjaxEvent('Purchase', {
        ...segmentation,
        errors: 'User Error',
        ...purchaseErrs,
      });
    }
  }

  function handlePurchaseResponseFailed(jqXHR) {
    this.phase = 'pay';
    if (jqXHR.statusText === 'abort') return;
    let errTitle = app.polyglot.t('purchase.errors.orderError');
    let errMsg = (jqXHR.responseJSON && jqXHR.responseJSON.reason) || '';

    if (jqXHR.responseJSON
      && jqXHR.responseJSON.code === 'ERR_INSUFFICIENT_INVENTORY'
      && typeof jqXHR.responseJSON.remainingInventory === 'number') {
      this.inventory = jqXHR.responseJSON.remainingInventory / coinDivisibility;
      errTitle = app.polyglot.t('purchase.errors.insufficientInventoryTitle');
      errMsg = app.polyglot.t('purchase.errors.insufficientInventoryBody', {
        smart_count: this.inventory,
        remainingInventory: new Intl.NumberFormat(app.settings.get('localCurrency'), {
          minimumFractionDigits: 0,
          maximumFractionDigits: 8,
        }).format(this.inventory),
      });
      // if (this.inventoryFetch) this.inventoryFetch.abort();
    } else if (errMsg === ERROR_DUST_AMOUNT) {
      errMsg = app.polyglot.t('purchase.errors.serverErrorBelowDust');
    }

    openSimpleMessage(errTitle, errMsg);
  }

  function formatPurchaseError() {
    const purchaseErrs = {};
    Object.keys(this.order.validationError).forEach((errKey) => {
      const domKey = errKey.replace(/\[[^\[\]]*\]/g, '').replace('.', '-');
      let container = this.$(`.js-${domKey}-errors`);
      // if no container exists, use the generic container
      container = container.length ? container : this.getCachedEl('.js-errors');
      const err = this.order.validationError[errKey];
      insertErrors(container, err);
      purchaseErrs[`UserError-${domKey}`] = err.join(', ');
    });
    return purchaseErrs;
  }

  function insertErrors(container, errors = []) {
    loadTemplate('formError.html', (t) => {
      container.html(t({
        errors,
      }));
    });
  }

  function completePurchase(data) {
    this.complete.orderID = data.orderID;
    this.phase = 'complete';
  }

  function remove() {
    if (this.orderSubmit) this.orderSubmit.abort();
    // if (this.inventoryFetch) this.inventoryFetch.abort();
    clearTimeout(this.quantityKeyUpTimer);
    super.remove();
  }

  function render() {
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

    let context = {
      ...this.order.toJSON(),
      phase: this.phase,
      listings: this.listings,
      listing: this.listing.toJSON(),
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
      phaseClass: `phase${capitalize(this.phase)}`,
      hasCoupons: this.listing.get('coupons').length
        && this.listing.get('metadata').get('contractType') !== 'CRYPTOCURRENCY',
    };
    ob = reactive({ ...window.templateHelpers, ...(context || {}) });

    this._$couponField = null;

    this.actionBtn.delegateEvents();
    this.actionBtn.setState({ phase: this.phase }, { renderOnChange: false });
    this.$('.js-actionBtn').append(this.actionBtn.render().el);

    // this.receipt.delegateEvents();
    // this.$('.js-receipt').append(this.receipt.render().el);

    // const receipt = createApp(Receipt, {
    //   model: this.order,
    //   listing: this.listing.toJSON(),
    //   prices: this.prices,
    //   coupons: this.couponObj,
    //   showTotalTip: this.getState().phase === 'pay',
    // });
    // receipt.mount(this.$('.js-receipt'));

    this.coupons.delegateEvents();
    this.$('.js-couponsWrapper').html(this.coupons.render().el);

    this.moderators.delegateEvents();
    this.$('.js-moderatorsWrapper').append(this.moderators.el);

    if (this.directPayment) this.directPayment.remove();
    this.directPayment = this.createChild(DirectPayment, {
      initialState: {
        active: !this.isModerated,
      },
    });
    this.listenTo(this.directPayment, 'click', () => this.handleDirectPurchaseClick());
    this.$('.js-directPaymentWrapper').append(this.directPayment.render().el);

    this.cryptoCurSelector.delegateEvents();
    this.$('.js-cryptoCurSelectorWrapper').append(this.cryptoCurSelector.render().el);

    if (this.shipping) {
      this.shipping.delegateEvents();
      this.$('.js-shippingWrapper').append(this.shipping.render().el);
    }

    // if this is a re-render, and the payment exists, render it
    if (this.payment) {
      this.payment.delegateEvents();
      this.$('.js-pending').append(this.payment.render().el);
    }

    this.complete.delegateEvents();
    this.$('.js-complete').append(this.complete.render().el);

    if (this.feeChange) this.feeChange.remove();
    this.feeChange = this.createChild(FeeChange);
    this.$('.js-feeChangeContainer').html(this.feeChange.render().el);

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

      this.$('#cryptoAmountCurrency').select2({ minimumResultsForSearch: Infinity });
    }

    this._renderedHash = this.listing.get('hash');

    return this;
  }
</script>
<style lang="scss" scoped>
</style>