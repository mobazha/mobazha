<template>
  <div class="modal purchase modalScrollPage" :key="viewKey">
    <BaseModal :modalInfo="{ showCloseButton: false }">
      <template v-slot:component>
        <div ref="popInMessages" class="popInMessageHolder js-popInMessages"></div>

        <div class="topControls gutterHSm flex">
          <template v-if="vendor">
            <div class="contentBox clrP clrSh3 clrBr clrT">
              <div class="padSm gutterHSm overflowAuto margRSm flexVCent">
                <a class="clrBr2 clrSh1 discTn flexNoShrink" :style="ob.getAvatarBgImage(vendor.avatarHashes)"></a>
                <p class="txUnl tx3 clamp">{{ vendor.name }}</p>
                <a class="link flexNoShrink tx6" @click="clickGoToListing">{{
                  origin === 'ShoppingCart' ? ob.polyT('purchase.returnToCart') : ob.polyT('purchase.returnToListing')
                }}</a>
              </div>
            </div>
          </template>
        </div>

        <div :class="`flexRow gutterH mainSection ${ob.phaseClass}`">
          <div class="col9">
            <div class="flexColRow gutterV">
              <template v-for="(listing, idx) in ob.listings" :key="listing.slug">
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
                          <template v-for="variant in itemsInfo[idx].variants" :key="variant.name">
                            <div class="width100 noOverflow">
                              <span class="clrT2">{{ variant.name }}: {{ variant.value }}</span>
                            </div>
                          </template>
                        </div>
                      </div>
                      <template v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                        <div class="flexNoShrink">
                          <div class="flexVCent gutterH purchaseQuantity">
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
                                v-model="formData.itemsData[idx].quantity"
                                @keyup="keyupQuantity(idx)"
                                placeholder="0"
                                data-var-type="bignumber"
                              />
                            </div>
                          </div>
                        </div>
                      </template>
                      <div class="pad flexNoShrink">
                        <b>{{ ob.currencyMod.convertAndFormatCurrency(totalPrice(idx), pricingCurrency(idx), displayCurrency) }}</b>
                      </div>
                    </div>

                    <OptionalFeatureLine :optionalFeatures="itemsInfo[idx].optionalFeatures" :pricingCurrency="pricingCurrency(idx)" :displayCurrency="displayCurrency" />

                    <div class="col6">
                      <template v-if="hasCoupons(listing) && ob.phase === 'pay'">
                        <div class="rowTn">
                          <label for="couponCode" class="tx5">{{ ob.polyT('purchase.couponCode') }}</label>
                        </div>
                        <div class="flex gutterH row">
                          <input
                            class="btnHeight clrBr clrP"
                            type="text"
                            id="couponCode"
                            @keyup.enter="applyCoupon(idx)"
                            v-model="formData.itemsData[idx].couponCode"
                            :placeholder="ob.polyT('purchase.couponCodePlaceholder')"
                          />
                          <button class="btn clrP clrBr clrSh2 flexNoShrink" @click="applyCoupon(idx)">
                            {{ ob.polyT('purchase.applyCode') }}
                          </button>
                        </div>
                        <div class="js-couponsWrapper">
                          <Coupons
                            ref="coupons"
                            :options="{
                              coupons: listing.coupons,
                              listingPrice: this.prices[idx].price,
                            }"
                            @changeCoupons="changeCoupons(idx, $event)"
                          />
                          <!-- // coupons are inserted here after they are added by the user. -->
                        </div>
                      </template>
                    </div>
                  </template>

                  <template v-else>
                    <div class="flexVCent gutterHLg row cryptoTitleWrap">
                      <div ref="cryptoTitle" :class="`js-cryptoTitle ${ob.phase !== 'pay' && ob.phase !== 'processing' ? 'flexExpand' : ''}`">
                        <CryptoTradingPairWrap
                          :options="{
                            tradingPairClass: 'cryptoTradingPairXL',
                            exchangeRateClass: 'clrT2 tx6',
                            fromCur: listing.get('metadata').get('acceptedCurrencies')[0],
                            toCur: listing.get('item').get('cryptoListingCurrencyCode'),
                          }"
                        />
                      </div>
                      <template v-if="ob.phase === 'pay' || ob.phase === 'processing'">
                        <div class="flexExpand">
                          <div class="flexVCent gutterHLg">
                            <label for="cryptoAmount" class="clrT txB required">{{ ob.polyT('purchase.cryptoAmount') }}</label>
                            <div class="inputSelect">
                              <input
                                type="number"
                                class="clrBr clrP clrSh2"
                                id="cryptoAmount"
                                @change="onChangeCryptoAmount"
                                :value="ob.quantity"
                                @keyup="keyupQuantity(idx)"
                                placeholder="0.0000"
                                size="8"
                                data-var-type="bignumber"
                              />
                              <template v-if="displayCurrency !== listing.item.cryptoListingCurrencyCode">
                                <Select2
                                  id="cryptoAmountCurrency"
                                  v-model="cryptoAmountCurrency"
                                  @change="changeCryptoAmountCurrency(idx)"
                                  class="clrBr clrP nestInputRight"
                                >
                                  <option
                                    v-for="cur in [listing.item.cryptoListingCurrencyCode, displayCurrency]"
                                    :key="cur"
                                    :value="cur"
                                    :selected="cur === cryptoAmountCurrency"
                                  >
                                    {{ cur }}
                                  </option>
                                </Select2>
                              </template>
                            </div>
                          </div>
                        </div>
                      </template>
                      <div class="pad flexNoShrink">
                        <CryptoPrice
                          :options="{
                            priceAmount: totalPrice(idx),
                            priceCurrencyCode: pricingCurrency(idx),
                            displayCurrency: displayCurrency,
                            priceModifier: listing.item.cryptoListingPriceModifier,
                          }"
                        />
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
                      <input
                        type="text"
                        id="purchaseCryptoAddress"
                        @change="changeCryptoAddress"
                        :value="ob.items[0].paymentAddress"
                        :placeholder="ob.polyT('purchase.cryptoAddressPlaceholder', { coinType: coinName })"
                        class="clrBr clrP rowSm"
                        :maxlength="ob.itemConstraints.maxPaymentAddressLength"
                      />
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
              <template v-if="shippingOptions && shippingOptions.length">
                <section class="contentBox padMd clrP clrBr clrSh3 js-shipping">
                  <div class="js-shipping-errors js-items-shipping-errors">
                    <FormError v-if="errors['shipping']" :errors="errors['shipping']" />
                    <FormError v-if="errors['items-shipping']" :errors="errors['items-shipping']" />
                  </div>
                  <Shipping
                    ref="shipping"
                    v-if="shippingOptions.length"
                    :options="{
                      getTotalShippingPrice: totalShippingPriceFunc,
                    }"
                    :bb="
                      function () {
                        return {
                          model: shippingOptions,
                        };
                      }
                    "
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
                        controlType: 'radio',
                        currencies,
                        disabledCurs: currencies.filter((c) => !isSupportedWalletCur(c)),
                        sort: false,
                      }"
                      v-model:activeCurs="formData.activeCurs"
                      @currencyClicked="onCurrencyClicked"
                    />
                  </div>
                </div>
              </section>
              <section class="contentBox padMd clrP clrBr clrSh3">
                <div class="flexColRows gutterVSm">
                  <div class="flexVCentClearMarg">
                    <h2 class="h4 flexExpand required">{{ ob.polyT('purchase.paymentTypeTitle') }}</h2>
                    <template v-if="showModerators">
                      <input type="checkbox" id="purchaseVerifiedOnly" v-model="showVerifiedOnly" />
                      <label class="tx5b" for="purchaseVerifiedOnly">{{ ob.polyT('settings.storeTab.verifiedOnly') }}</label>
                    </template>
                  </div>
                  <template v-if="showModerators">
                    <div class="js-moderated-errors">
                      <FormError v-if="errors['moderated']" :errors="errors['moderated']" />
                    </div>
                    <div ref="moderatorsWrapper" class="js-moderatorsWrapper">
                      <Moderators
                        ref="moderators"
                        :options="{
                          moderatorIDs: moderatorIDs,
                          useCache: false,
                          fetchErrorTitle: ob.polyT('purchase.errors.moderatorsTitle'),
                          fetchErrorMsg: ob.polyT('purchase.errors.moderatorsMsg'),
                          purchase: true,
                          cardState: 'unselected',
                          notSelected: 'unselected',
                          singleSelect: true,
                          radioStyle: true,
                          initialState: {
                            showOnlyCur: currencies[0],
                          },
                        }"
                        :showVerifiedOnly="showVerifiedOnly"
                        :modCurrency="paymentCoin"
                        @clickShowUnverified="showVerifiedOnly = false"
                        @cardSelect="onCardSelect"
                      />
                    </div>
                    <div>
                      <div class="clrT2 tx6 rowMd">{{ ob.polyT('purchase.moderatorsDisclaimer') }}</div>
                    </div>
                    <hr class="clrBr row" />
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
                        v-model="formData.emailAddress"
                        @blur="blurEmailAddress"
                        :placeholder="ob.polyT('purchase.emailPlaceholder')"
                      />
                    </div>
                    <div>
                      <span class="txSm clrT2">{{ ob.polyT('purchase.emailNote') }}</span>
                    </div>
                  </div>
                </div>
                <hr class="clrBr row" />
                <div class="rowTn">
                  <label for="memo" class="tx5">
                    {{ ob.polyT('purchase.memo') }}
                  </label>
                </div>
                <textarea
                  class="clrBr clrP js-purchaseField"
                  id="memo"
                  @blur="blurMemo"
                  maxlength="5000"
                  rows="6"
                  :placeholder="ob.polyT('purchase.memoPlaceholder')"
                  v-model="formData.itemsData[0].memo"
                ></textarea>
              </section>
            </template>
            <template v-if="ob.phase === 'pending'">
              <section ref="pendingPayment" class="contentBox padMd clrP clrBr clrSh3 js-pending">
                <Payment
                  v-if="paymentData"
                  :options="{
                    balanceRemaining: curDefToDecimal(paymentData.amount),
                    paymentAddress: paymentData.paymentAddress,
                    orderID: paymentData.orderID,
                    isModerated: !!this.order.get('moderator'),
                    metricsOrigin: 'Purchase',
                    paymentCoin,
                  }"
                  @walletPaymentComplete="completePurchase"
                />
              </section>
            </template>
            <template v-if="ob.phase === 'complete'">
              <section class="contentBox padMd clrP clrBr clrSh3 js-complete">
                <Complete
                  :options="{
                    vendor,
                    orderID,
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
                  :phase="ob.phase"
                  :outdatedHash="outdatedHash"
                  :bb="
                    function () {
                      return {
                        oneListing,
                      };
                    }
                  "
                  @purchase="purchaseListing"
                  @close="close"
                  @reloadOutdated="onReloadOutdated"
                />
              </div>
              <div class="rowLg">
                <!-- <div class="js-receipt"></div> -->
                <Receipt
                  v-if="order"
                  :key="orderKey"
                  :options="{
                    prices,
                    coupons: couponObj,
                    showTotalTip: _state.phase === 'pay',
                    totalShippingPrice: selectedShippingPrice,
                  }"
                  :bb="
                    function () {
                      return {
                        model: order,
                        listing: oneListing,
                      };
                    }
                  "
                />
                <template v-if="showModerators">
                  <hr class="clrBr" />
                  <div class="padSm txSm txCtr clrT2">
                    {{ ob.polyT('purchase.moderatorNote') }}
                  </div>
                </template>
              </div>
              <div ref="feeChangeContainer" class="tx6 js-feeChangeContainer">
                <FeeChange />
              </div>
            </section>
          </div>
        </div>
      </template>
    </BaseModal>
    <Teleport to="#js-vueModal">
      <Settings v-if="showSettings" :options="{ initialTab: 'Addresses' }" @close="closeSettings" />
    </Teleport>
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
// import {
//   getInventory,
//   events as inventoryEvents,
// } from '../../../utils/inventory';
import { startAjaxEvent, endAjaxEvent } from '../../../../backbone/utils/metrics';
import { toStandardNotation } from '../../../../backbone/utils/number';
import { decimalToInteger, isValidCoinDivisibility, curDefToDecimal, getCoinDivisibility } from '../../../../backbone/utils/currency';
import { capitalize } from '../../../../backbone/utils/string';
import { events as outdatedListingHashesEvents } from '../../../../backbone/utils/outdatedListingHashes';
import { isSupportedWalletCur } from '../../../../backbone/data/walletCurrencies';
import Order from '../../../../backbone/models/purchase/Order';
import Item from '../../../../backbone/models/purchase/Item';
import OrderListings from '../../../../backbone/collections/OrderListings';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import PopInMessage, { buildRefreshAlertMessage } from '../../../../backbone/views/components/PopInMessage';

import ActionBtn from './ActionBtn.vue';
import Complete from './Complete.vue';
import Coupons from './Coupons.vue';
import DirectPayment from './DirectPayment.vue';
import Payment from './Payment.vue';
import Receipt from './Receipt.vue';
import Shipping from './Shipping.vue';

import Settings from '@/views/modals/settings/Settings.vue';

export default {
  components: {
    ActionBtn,
    Complete,
    Coupons,
    Receipt,
    DirectPayment,
    Payment,
    Shipping,
    Settings,
  },
  props: {
    options: {
      type: Object,
      default: {
        itemsInfo: [],
        vendor: {},
        phase: 'pay',
      },
    },
    bb: Function,
  },
  data() {
    return {
      viewKey: 0,

      formData: {
        itemsData: [
          {
            quantity: 0,
            memo: '',
            couponCode: '',

            coupons: [],
          },
        ],
        activeCurs: [],
        emailAddress: '',
      },

      _state: {
        phase: 'pay',
      },

      cart: {},
      vendor: {},
      order: undefined,
      orderKey: 0,

      oneListing: undefined,
      listings: undefined,
      moderators: undefined,
      couponObj: [],

      shippingOptions: undefined,

      cryptoAmountCurrency: '',
      _cryptoQuantity: 0,

      coinName: '',
      moderatorIDs: [],
      showVerifiedOnly: true,
      shipping: {
        selectedAddress: '',
      },
      shippingOptionKey: 0,

      outdatedHash: false,

      orderID: '',
      showModerators: false,
      isModerated: false,

      showSettings: false,

      paymentData: undefined,

      errors: {},
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {},
  unmounted() {
    if (this.orderSubmit) this.orderSubmit.abort();
    if (this.inventoryFetch) this.inventoryFetch.abort();
    clearTimeout(this.quantityKeyUpTimer);
  },
  computed: {
    ob() {
      const item = this.order.get('items').at(0);
      let uiQuantity = item ? item.get('quantity') : 0;

      if (this.oneListing?.isCrypto && this._cryptoQuantity !== undefined) {
        uiQuantity = uiQuantity instanceof bigNumber && !uiQuantity.isNaN() ? toStandardNotation(this._cryptoQuantity) : this._cryptoQuantity;
      }

      return {
        ...this.templateHelpers,
        ...this.order.toJSON(),
        ...this._state,
        listings: this.itemsToPurchase.toJSON(),
        itemConstraints: this.order.get('items').at(0).constraints,
        quantity: uiQuantity,
        isCrypto: this.oneListing.isCrypto,
        phaseClass: `phase${capitalize(this._state.phase)}`,
      };
    },
    helperMessage() {
      const warning =
        this.phase === 'pay' || this.phase === 'processing'
          ? `<b>${ob.polyT('purchase.cryptoAddressHelperWarning')}</b>`
          : `<b>${ob.polyT('purchase.cryptoAddressHelperWarning2')}</b>`;

      return ob.polyT('purchase.cryptoAddressHelper', {
        name: this.vendor.name,
        coinType: this.coinName,
        warning,
      });
    },
    paymentCoin() {
      return this.formData.activeCurs[0];
    },
    prices() {
      let access = this.orderKey;

      // return an array of price objects that matches the items in the order
      return this.order.get('items').map((item, idx) => {
        const shipping = item.get('shipping');
        const sName = shipping.get('name');
        const sService = shipping.get('service');
        const sOpt = this.shippingOptions.findWhere({ name: sName });
        const sOptService = sOpt ? sOpt.get('services').findWhere({ name: sService }) : '';

        const options = item.get('options').toJSON();
        const selections = options.map((option) => ({
          option: option.name,
          variant: option.value,
        }));
        const listing = this.itemsToPurchase.get(item.id);
        const sku = listing
          .get('item')
          .get('skus')
          .find((v) => _.isEqual(v.get('selections'), selections));

        const optionalFeatures = this.itemsInfo[idx].optionalFeatures || [];
        let oPrice = bigNumber(0);
        optionalFeatures.forEach((feature) => {
            oPrice = oPrice.plus(feature.surcharge);
        });

        return {
          title: listing.get('item').get('title'),
          price: bigNumber(listing.price.amount),
          sPrice: bigNumber(sOptService ? sOptService.get('firstFreight') || 0 : 0),
          vPrice: bigNumber(sku ? sku.get('surcharge') || 0 : 0),
          oPrice,
          quantity: bigNumber(item.get('quantity')),
          currency: listing.price.currencyCode,
        };
      });
    },
    displayCurrency() {
      return app.settings.get('localCurrency');
    },
    totalShippingPriceFunc() {
      return this.getTotalShippingPrice.bind(this);
    },
    selectedShippingPrice() {
      let access1 = this.formData.itemsData[0].quantity;
      let access2 = this.shippingOptionKey;

      const item = this.order.get('items').at(0);

      const shipping = item.get('shipping');
      if (!shipping) {
        return bigNumber(0);
      }

      const sName = shipping.get('name');
      const sService = shipping.get('service');

      return this.getTotalShippingPrice(sName, sService);
    },

    currencies() {
      let currencies = this.oneListing.get('metadata').get('acceptedCurrencies') || [];
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
    curDefToDecimal,

    totalPrice(i) {
      return this.prices[i].price.plus(this.prices[i].vPrice);
    },
    pricingCurrency(i) {
      return this.prices[i].currency;
    },

    getListingCoinDivisibility(listing) {
      let currencyCode;
      try {
        currencyCode = listing.isCrypto ? listing.get('item').cryptoListingCurrencyCode : listing.get('metadata').get('pricingCurrency').code;
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

    getTotalShippingPrice(shippingOptionName, shippingServiceName) {
      const sOpt = this.shippingOptions.findWhere({ name: shippingOptionName });
      const sOptService = sOpt ? sOpt.get('services').findWhere({ name: shippingServiceName }) : '';

      if (!sOpt || !sOptService || sOpt.type === 'LOCAL_PICKUP') {
        return { price: bigNumber(0), currency: undefined };
      }

      const sOption = sOpt.toJSON();
      const sService = sOptService.toJSON();

      let gramsTotal = bigNumber(0);
      this.order.get('items').forEach((item) => {
        const listing = this.itemsToPurchase.get(item.id);
        const itemGrams = listing.get('item').get('grams');

        gramsTotal = gramsTotal.plus(bigNumber(itemGrams).times(bigNumber(item.get('quantity'))));
      });
      if (gramsTotal.eq(bigNumber(0))) {
        return { price: bigNumber(0), currency: sOption.currency };
      }

      const firstFreight = bigNumber(sService.firstFreight);
      let renewalFee = bigNumber(0);
      if (sOption.serviceType === 'FIRST_RENEWAL_FEE') {
        if (gramsTotal.gt(bigNumber(sService.firstWeight))) {
          const unitAmount = gramsTotal.minus(bigNumber(sService.firstWeight)).div(bigNumber(sService.renewalUnitWeight).integerValue(bigNumber.ROUND_CEIL));
          renewalFee = bigNumber(sService.renewalUnitPrice).times(unitAmount);
        }
      }

      return { price: firstFreight.plus(renewalFee).plus(bigNumber(sService.registrationFee)), currency: sOption.currency };
    },

    hasCoupons(listing) {
      return listing && listing?.coupons.length && listing?.metadata.contractType !== 'CRYPTOCURRENCY';
    },

    loadData(options = {}) {
      if (!this.itemsToPurchase || !(this.itemsToPurchase instanceof OrderListings)) {
        throw new Error('Please provide a OrderListings model');
      }

      if (!options.vendor) {
        throw new Error('Please provide a vendor object');
      }

      this.baseInit(options);

      this._state.phase = 'pay';

      this.oneListing = this.itemsToPurchase.at(0);

      this.shippingOptions = this.oneListing.get('shippingOptions');
      const moderatorIDs = this.oneListing.get('moderators') || [];
      const disallowedIDs = [app.profile.id, this.vendor.peerID];
      this.moderatorIDs = _.without(moderatorIDs, ...disallowedIDs);

      this.showModerators = this.moderatorIDs.length > 0;

      this.couponObj = new Array(this.itemsToPurchase.length).fill([]);

      this.order = new Order(
        {},
        {
          shippable: !!(this.shippingOptions && this.shippingOptions.length),
          moderated: this.moderatorIDs.length && app.verifiedMods.matched(this.moderatorIDs).length,
        }
      );

      /*
         to support multiple items in a purchase in the future, pass in listings in the options,
         and add them to the order as items here.
      */
      this.formData.itemsData = [];
      this.itemsToPurchase.forEach((listing, i) => {
        const item = new Item(
          {
            listingHash: listing.get('hash'),
            quantity: this.itemsInfo[i].quantity ? bigNumber(this.itemsInfo[i].quantity) : bigNumber('1'),
            options: this.itemsInfo[i].variants || [], // Need update to the selected listing variants for each listing
            optionalFeatures: this.itemsInfo[i].optionalFeatures?.map(item => item.name) || [],
          },
          {
            isCrypto: listing.isCrypto,
            // inventory: () =>
            //   (
            //     typeof this.inventory === 'number' ?
            //       this.inventory : 99999999999999999
            //   ),
            getCoinDiv: () => this.getListingCoinDivisibility(listing),
            getCoinType: () => listing.get('metadata').get('coinType'),
          }
        );
        // add the item to the order.
        this.order.get('items').add(item);

        this.formData.itemsData.push({
          quantity: item.get('quantity'),
        });
      });

      let currencies = this.oneListing.get('metadata').get('acceptedCurrencies') || [];
      this.formData.activeCurs = currencies.length && this.oneListing.isCrypto ? [currencies[0]] : [];

      this.cryptoAmountCurrency = this.oneListing.get('item').get('cryptoListingCurrencyCode');

      // If the parent has the inventory, pass it in, otherwise we'll fetch it.
      // -- commenting out for now since inventory is not functioning properly on the server
      // this.inventory = this.options.inventory;
      // if (
      //   this.oneListing.isCrypto &&
      //   typeof this.inventory !== 'number'
      // ) {
      //   this.inventoryFetch = getInventory(
      //     this.oneListing.get('vendorID').peerID,
      //     {
      //       slug: this.oneListing.get('slug'),
      //       coinDivisibility:
      //         this.oneListing.get('metadata')
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

      this._latestHash = this.oneListing.get('hash');
      this._renderedHash = null;

      this.listenTo(outdatedListingHashesEvents, 'newHash', (e) => {
        this._latestHash = e.oldHash;
        if (e.oldHash === this._renderedHash) this.outdateHash();
      });
    },

    onCurrencyClicked(cOpts) {
      if (cOpts.active) this.$refs.moderators.setState({ showOnlyCur: cOpts.currency });
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

    showDataChangedMessage() {
      if (this.dataChangePopIn && !this.dataChangePopIn.isRemoved()) {
        this.dataChangePopIn.$el.velocity('callout.shake', { duration: 500 });
      } else {
        this.dataChangePopIn = this.createChild(PopInMessage, {
          messageText: buildRefreshAlertMessage(app.polyglot.t('purchase.purchaseDataChangedPopin')),
        });

        this.listenTo(this.dataChangePopIn, 'clickRefresh', () => {
          this.viewKey += 1;
        });

        this.listenTo(this.dataChangePopIn, 'clickDismiss', () => {
          this.dataChangePopIn.remove();
          this.dataChangePopIn = null;
        });

        $(this.$refs.popInMessages).append(this.dataChangePopIn.render().el);
      }
    },

    goToListing() {
      app.router.navigate(`${this.vendor.peerID}/store/${this.oneListing.get('slug')}`, { trigger: true });
      this.close();
    },

    clickGoToListing() {
      if (this.origin === 'ShoppingCart') {
        this.close();
        return;
      }
      this.goToListing();
    },

    clickClose() {
      this.$emit('closeBtnPressed');
      this.close();
    },

    handleDirectPurchaseClick() {
      if (!this.isModerated) return;

      if (this.$refs.moderators) {
        this.$refs.moderators.deselectOthers();
        this.isModerated = this.$refs.moderators.selectedIDs.length > 0;
      }
    },

    onCardSelect() {
      this.isModerated = this.$refs.moderators.selectedIDs.length > 0;
    },

    changeCryptoAddress(e) {
      this.order.get('items').at(0).set('paymentAddress', e.target.value);
    },

    setModelQuantity(idx, quantity) {
      let cur = this.cryptoAmountCurrency;

      if (this.oneListing.isCrypto && (typeof cur !== 'string' || !cur)) {
        throw new Error('Please provide the currency code as a valid, non-empty string.');
      }

      this.order.get('items').at(idx).set({ quantity });

      this.orderKey += 1;
    },

    onChangeCryptoAmount(e) {
      this._cryptoQuantity = e.target.value;
    },

    changeCryptoAmountCurrency(idx) {
      this.setModelQuantity(idx, this._cryptoQuantity);
    },

    keyupQuantity(idx) {
      // wait until they stop typing
      if (this.quantityKeyUpTimer) {
        clearTimeout(this.quantityKeyUpTimer);
      }

      this.quantityKeyUpTimer = setTimeout(() => {
        let { quantity } = this.formData.itemsData[idx];
        if (quantity != null) {
          quantity = bigNumber(quantity);
        }
        if (this.oneListing.isCrypto) this._cryptoQuantity = quantity;
        this.setModelQuantity(idx, quantity);
      }, 150);
    },

    clickNewAddress() {
      this.showSettings = true;
    },

    closeSettings() {
      this.showSettings = false;
    },

    applyCoupon(idx) {
      this.$refs.coupons[idx].addCode(this.formData.itemsData[idx].couponCode).then((result) => {
        // if the result is valid, clear the input field
        if (result.type === 'valid') {
          this.formData.itemsData[idx].couponCode = '';
        }
      });
    },

    blurEmailAddress() {
      this.order.set('alternateContactInfo', this.formData.emailAddress);
    },

    blurMemo() {
      this.order.get('items').at(0).set('memo', this.formData.itemsData[0].memo);
    },

    changeCoupons(idx, $event) {
      const { hashes, codes } = $event;

      // combine the codes and hashes so the receipt can check both.
      // if this is the user's own listing they will have codes instead of hashes
      const hashesAndCodes = hashes.concat(codes);
      const filteredCoupons = this.itemsToPurchase
        .at(idx)
        .get('coupons')
        .filter((coupon) => hashesAndCodes.indexOf(coupon.get('hash') || coupon.get('discountCode')) !== -1);
      this.couponObj[idx] = filteredCoupons.map((coupon) => coupon.toJSON());

      this.order.get('items').at(idx).set('coupons', codes);
    },

    updateShippingOption(selectedOption) {
      // Set the shipping option.
      this.order.get('items').forEach((item) => {
        item.get('shipping').set(selectedOption);
      });

      this.shippingOptionKey += 1;
    },

    outdateHash() {
      this.outdatedHash = true;
    },

    purchaseListing() {
      // Clear any old errors.
      this.errors = {};

      // Don't allow a zero or negative price purchase.
      const priceObj = this.prices[0];
      if (priceObj.price.plus(priceObj.vPrice).plus(priceObj.sPrice).lte(0)) {
        this.insertErrors('js-errors', [app.polyglot.t('purchase.errors.zeroPrice')]);
        this.setState({ phase: 'pay' });
        return;
      }

      // Set the payment coin.
      let paymentCoin = this.paymentCoin;
      this.order.set({ paymentCoin });

      // Set the shipping address if the listing is shippable.
      if (this.$refs.shipping && this.$refs.shipping.selectedAddress) {
        this.order.addAddress(this.$refs.shipping.selectedAddress);
      }

      // Set the moderator.
      const moderator = (this.$refs.moderators && this.$refs.moderators.selectedIDs.length > 0 && this.$refs.moderators.selectedIDs[0]) || '';
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
        if (this.oneListing.isOwnListing) {
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
          const coinDivisibility = this.getListingCoinDivisibility(this.oneListing);
          const cryptoItems = [];

          if (this.oneListing.isCrypto) {
            if (!isValidCoinDivisibility(coinDivisibility)[0]) {
              this.setState({ phase: 'pay' });
              openSimpleMessage(app.polyglot.t('purchase.errors.genericPurchaseErrTitle'), app.polyglot.t('purchase.errors.invalidCoinDiv'));
              return;
            }

            try {
              const items = this.order.get('items');
              for (let i = 0; i < items.length; i += 2) {
                const item = items.at(i);
                cryptoItems.push({
                  ...item.toJSON(),
                  quantity: decimalToInteger(item.get('quantity'), coinDivisibility),
                });
              }
            } catch (e) {
              this.setState({ phase: 'pay' });
              openSimpleMessage(app.polyglot.t('purchase.errors.genericPurchaseErrTitle'), app.polyglot.t('purchase.errors.unableToConvertCryptoQuantity'));
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
              items: this.oneListing.isCrypto ? cryptoItems : this.order.get('items').toJSON(),
            },
            'cid'
          );

          $.post({
            url: app.getServerUrl('ob/purchase'),
            data: JSON.stringify(postData),
            dataType: 'json',
            contentType: 'application/json',
          })
            .done((data) => {
              this.setState({ phase: 'pending' });

              this.paymentData = data;

              endAjaxEvent('Purchase');
            })
            .fail((jqXHR) => {
              this.setState({ phase: 'pay' });
              if (jqXHR.statusText === 'abort') return;
              let errTitle = app.polyglot.t('purchase.errors.orderError');
              let errMsg = (jqXHR.responseJSON && jqXHR.responseJSON.reason) || '';

              if (jqXHR.responseJSON && jqXHR.responseJSON.code === 'ERR_INSUFFICIENT_INVENTORY' && typeof jqXHR.responseJSON.remainingInventory === 'number') {
                this.inventory = jqXHR.responseJSON.remainingInventory / coinDivisibility;
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

    insertErrors(container, errors = []) {
      this.errors[container] = errors;
    },

    completePurchase(data) {
      this.orderID = data.orderID;

      this.setState({ phase: 'complete' });
    },

    render() {
      this._renderedHash = this.oneListing.get('hash');

      return this;
    },
  },
};
</script>
<style lang="scss" scoped>
.purchaseQuantity {
  input[type='number'] {
    width: 100px;
  }
}
</style>
