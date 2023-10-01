<template>
  <div class="receipt flexColRows gutterVSm tx5b">
    <hr class="clrBr">
    <b>{{ ob.polyT('purchase.receipt.summary') }}</b>

    <template v-for="(priceObj, i) in prices" :key="i">
      <div class="flexRow gutterHSm">
        <span class="flexExpand">
          {{
            !listing.isCrypto ? ob.polyT('purchase.receipt.listing')
            : ob.polyT('purchase.receipt.cryptoIconsAmountCombo', {
              icons: ob.crypto?.tradingPair({
                className: 'cryptoTradingPairSm cryptoTradingPair',
                fromCur: listing?.toJSON().metadata.acceptedCurrencies[0],
                toCur: listing?.toJSON().item.cryptoListingCurrencyCode,
                truncateCurAfter: 5,
              }),
              cryptoIconsAmount:
                `<span class="cryptoQuantity">${ob.polyT('purchase.receipt.cryptoIconsAmount',
                  { amount: ob.number.toStandardNotation(priceObj.quantity) })}</span>`,
            })
          }}
        </span>
        <div class="constrainedWidth">
          <div class="flexHRight">
            <b>
              {{
                !listing.isCrypto
                ? (priceObj.preCouponPrice ? ob.currencyMod.formatCurrency(priceObj.preCouponPrice, viewingCurrency) : 0)
                : (priceObj.subTotal ? ob.currencyMod.formatCurrency(priceObj.subTotal.plus(priceObj.shippingTotal), viewingCurrency) : 0)
              }}
            </b>
          </div>
        </div>
      </div>
      <template v-for="(coupon, j) in coupons" :key="j">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">
            {{ ob.polyT('purchase.receipt.coupon') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ coupon.percentDiscount ? `-${coupon.percentDiscount}%`
                  : (coupon.priceDiscount && !coupon.priceDiscount.isNaN() ?
                    `-${convertAndFormatCurrency(coupon.priceDiscount, listingCurrency, viewingCurrency)}` : "") }}
              </b>
            </div>
          </div>
        </div>
      </template>
      <div
        v-if="listing.toJSON().shippingOptions && listing.toJSON().shippingOptions.length && priceObj.shippingPrice !== priceObj.additionalShippingPrice && priceObj.quantity > 1">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">
            {{ ob.polyT('purchase.receipt.shipping') }}
          </span>
        </div>
        <div class="flexRow subShipping gutterHSm">
          <span class="flexExpand">
            {{ ob.polyT('purchase.receipt.firstItem') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ ob.currencyMod.formatCurrency(priceObj.shippingPrice, viewingCurrency) }}
              </b>
            </div>
          </div>
        </div>
        <div class="flexRow subShipping gutterHSm">
          <span class="flexExpand">
            {{ ob.polyT('purchase.receipt.additionalItems') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ priceObj.additionalShippingPrice ? ob.currencyMod.formatCurrency(priceObj.additionalShippingPrice, viewingCurrency) : 0 }}
              </b>
            </div>
          </div>
        </div>
      </div>

      <template v-else-if="listing.toJSON().shippingOptions && listing.toJSON().shippingOptions.length">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">
            {{ ob.polyT('purchase.receipt.shipping') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ priceObj.shippingPrice ? ob.currencyMod.formatCurrency(priceObj.shippingPrice, viewingCurrency) : 0 }}
              </b>
            </div>
          </div>
        </div>
      </template>
      <hr class="clrBr">
      <template v-if="priceObj.quantity && priceObj.quantity.gt(0)">
        <template v-if="!listing.isCrypto">
          <div class="flexRow gutterHSm">
            <span class="flexExpand">
              {{ ob.polyT('purchase.receipt.subtotal', { quantity: ob.number.toStandardNotation(priceObj.quantity) }) }}
            </span>
            <div class="constrainedWidth">
              <div class="flexHRight">
                <b>
                  {{ priceObj.subTotal ? ob.currencyMod.formatCurrency(priceObj.subTotal, viewingCurrency) : 0 }}
                </b>
              </div>
            </div>
          </div>
        </template>
        <template v-if="listing.toJSON().shippingOptions && listing.toJSON().shippingOptions.length && priceObj.shippingTotal">
          <div class="flexRow gutterHSm">
            <span class="flexExpand">
              {{ ob.polyT('purchase.receipt.shippingTotal') }}
            </span>
            <div class="constrainedWidth">
              <div class="flexHRight">
                <b>
                  {{ ob.currencyMod.formatCurrency(priceObj.shippingTotal, viewingCurrency) }}
                </b>
              </div>
            </div>
          </div>
        </template>
      </template>
      <div class="flexRow">
        <span class="flexExpand">
          {{ ob.polyT('purchase.receipt.total') }}
          <span class="toolTip clrTAlert" :data-tip="totalTip()" v-show="showTotalTip"><span
              class="ion-alert-circled padSm"></span></span>
        </span>
        <div class="constrainedWidth">
          <div class="flexHRight">
            <b>
              {{ priceObj.subTotal ? ob.currencyMod.formatCurrency(priceObj.subTotal.plus(priceObj.shippingTotal),
                viewingCurrency) : '' }}
            </b>
          </div>
        </div>
      </div>
    </template>

  </div>
</template>

<script>
import app from '../../../../backbone/app';
import { convertCurrency, getExchangeRate } from '../../../../backbone/utils/currency';
import bigNumber from 'bignumber.js';
// import {
//   getCoinDivisibility,
//   nativeNumberFormatSupported,
//   defaultCryptoCoinDivisibility,
// } from '../../../utils/currency';
import Order from '../../../../backbone/models/purchase/Order';
import Listing from '../../../../backbone/models/listing/Listing';

import * as templateHelpers from '../../../../backbone/utils/templateHelpers';


export default {
  mixins: [],
  props: {
    order: Object,
    listing: Object,
    prices: Array,
    paymentCoin: {
      type: String,
      default: '',
    },
    coupons: Array,
    showTotalTip: {
      type: Boolean,
      default: true,
    }
  },
  data () {
    return {
    };
  },
  created () {
    this.loadData(this.props);
  },
  mounted () {
  },
  computed: {
    listingCurrency () {
      return this.listing.get('metadata').get('pricingCurrency').code
    },
    viewingCurrency () {
      let displayCurrency = app.settings.get('localCurrency');
      let listingCurrency = this.listing.get('metadata').get('pricingCurrency').code

      return getExchangeRate(displayCurrency) !== undefined ? displayCurrency : listingCurrency;
    },

  },
  methods: {
    totalTip () {
      let totalTip = "";
      if (this.viewingCurrency !== this.listingCurrency) {
        totalTip = this.ob.polyT('purchase.receipt.totalWarning1', { currency: this.viewingCurrency });
      } else if (this.viewingCurrency !== this.paymentCoin) {
        if (this.paymentCoin) {
          totalTip = this.ob.polyT('purchase.receipt.totalWarning2', { currency: this.paymentCoin });
        } else {
          totalTip = this.ob.polyT('purchase.receipt.totalWarning2NoCoin');
        }
      }

      return totalTip;
    },

    loadData (opts = {}) {

      this.prices.forEach((priceObj, i) => {
        // convert the prices here, to prevent rounding errors in the display
        const basePrice = convertCurrency(priceObj.price, this.listingCurrency, this.viewingCurrency);

        priceObj.shippingPrice = convertCurrency(priceObj.sPrice, this.listingCurrency, this.viewingCurrency);

        priceObj.additionalShippingPrice = convertCurrency(priceObj.aPrice, this.listingCurrency, this.viewingCurrency);

        const surcharge = convertCurrency(priceObj.vPrice, this.listingCurrency, this.viewingCurrency);

        const validQuantity = priceObj.quantity && !priceObj.quantity.isNaN() && priceObj.quantity.gt(0);

        priceObj.quantity = validQuantity ? priceObj.quantity : bigNumber(1);
        if (this.listing.isCrypto) {
          priceObj.quantity = validQuantity ? priceObj.quantity : bigNumber(0);
        }

        let itemTotal = basePrice.plus(surcharge);
        priceObj.preCouponPrice = itemTotal;
        this.coupons.forEach((coupon) => {
          if (coupon.percentDiscount) {
            itemTotal = itemTotal.minus(
              itemTotal.times(0.01).times(coupon.percentDiscount)
            );
          } else if (coupon.priceDiscount && !coupon.priceDiscount.isNaN()) {
            const convertPriceDiscount =
              convertCurrency(
                coupon.priceDiscount,
                this.listingCurrency,
                this.viewingCurrency
              );
            itemTotal = itemTotal.minus(convertPriceDiscount);
          }
        });
        priceObj.subTotal = itemTotal.times(priceObj.quantity);
        priceObj.shippingTotal = priceObj.shippingPrice.plus(priceObj.additionalShippingPrice.times(priceObj.quantity.minus(1)));

        let quantity =
          priceObj.quantity &&
            !priceObj.quantity.isNaN() &&
            priceObj.quantity.gt(0) ?
            priceObj.quantity : bigNumber(1);

        if (this.listing.isCrypto) {
          quantity =
            priceObj.quantity &&
              !priceObj.quantity.isNaN() &&
              priceObj.quantity.gt(0) ?
              priceObj.quantity : bigNumber(0);
        }
        priceObj.quantity = quantity;
      });
    },

    updatePrices (prices) {
      this.prices = prices;
    },

  }
}
</script>
<style lang="scss" scoped></style>
