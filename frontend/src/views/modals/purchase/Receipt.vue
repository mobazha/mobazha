<template>
  <div class="receipt flexColRows gutterVSm tx5b">
    <hr class="clrBr">
    <b>{{ ob.polyT('purchase.receipt.summary') }}</b>

    <template v-for="(priceObj, i) in updatedPrice" :key="i">
      <div class="flexRow gutterHSm">
        <span class="flexExpand">
          <div v-if="!ob.isCrypto" v-html="`${priceObj.title} x ${ob.number.toStandardNotation(priceObj.quantity)}`" />
          <template v-else>
            <CryptoTradingPair :options="ob.crypto.tradingPairOptions({
              className: 'cryptoTradingPairSm cryptoTradingPair',
              fromCur: ob.listing.metadata.acceptedCurrencies[0],
              toCur: ob.listing.item.cryptoListingCurrencyCode,
              truncateCurAfter: 5,
            })" />
            <span class="cryptoQuantity">
              {{ ob.polyT('purchase.receipt.cryptoIconsAmount', { amount: ob.number.toStandardNotation(priceObj.quantity) }) }}
            </span>
          </template>
        </span>
        <div class="constrainedWidth">
          <div class="flexHRight">
            <b>
              {{
                !ob.isCrypto
                ? (priceObj.preCouponPrice ? ob.currencyMod.formatCurrency(priceObj.preCouponPrice, viewingCurrency) : 0)
                : (priceObj.subTotal ? ob.currencyMod.formatCurrency(priceObj.subTotal.plus(priceObj.shippingTotal), viewingCurrency) : 0)
              }}
            </b>
          </div>
        </div>
      </div>
      <template v-for="(coupon, j) in options.coupons[i]" :key="j">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">
            {{ ob.polyT('purchase.receipt.coupon') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ coupon.percentDiscount ? `-${coupon.percentDiscount}%`
                  : (coupon.priceDiscount && !coupon.priceDiscount.isNaN() ?
                    `-${convertAndFormatCurrency(coupon.priceDiscount, priceObj.currency, viewingCurrency)}` : "") }}
              </b>
            </div>
          </div>
        </div>
      </template>

      <hr class="clrBr">

      <template v-if="!ob.isCrypto">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">
            {{ ob.polyT('purchase.receipt.subtotal') }}
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

      <br/>

    </template>

    <hr class="clrBr">

    <template v-if="ob.listing.shippingOptions && ob.listing.shippingOptions.length && shippingTotal">
      <div class="flexRow gutterHSm">
        <span class="flexExpand">
          {{ ob.polyT('purchase.receipt.shippingTotal') }}
        </span>
        <div class="constrainedWidth">
          <div class="flexHRight">
            <b>
              {{ shippingTotal }}
            </b>
          </div>
        </div>
      </div>
    </template>

    <div class="flexRow">
      <span class="flexExpand">
        {{ ob.polyT('purchase.receipt.total') }}
        <span class="toolTip clrTAlert" :data-tip="totalTip" v-show="showTotalTip"><span
            class="ion-alert-circled padSm"></span></span>
      </span>
      <div class="constrainedWidth">
        <div class="flexHRight">
          <b>
            {{ ob.currencyMod.formatCurrency(totalPrice, viewingCurrency) }}
          </b>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import app from '../../../../backbone/app';
import { convertCurrency, convertAndFormatCurrency, getExchangeRate } from '../../../../backbone/utils/currency';
import bigNumber from 'bignumber.js';
// import {
//   getCoinDivisibility,
//   nativeNumberFormatSupported,
//   defaultCryptoCoinDivisibility,
// } from '../../../utils/currency';
import Order from '../../../../backbone/models/purchase/Order';
import Listing from '../../../../backbone/models/listing/Listing';

export default {
  props: {
    options: {
      type: Object,
      default: {
        prices: [],
        coupons: [],
        showTotalTip: true,
        totalShippingPrice: {price: 0, currency: ''},
      },
	  },
    bb: Function,
  },
  data () {
    return {
      paymentCoin: '',
      showTotalTip: true,
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      const displayCurrency = app.settings.get('localCurrency');

      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        listing: this.listing.toJSON(),
        displayCurrency,
        paymentCoin: this.paymentCoin,
        showTotalTip: this.showTotalTip,
        isCrypto: this.listing.isCrypto,
      };
    },
    displayCurrency() {
      return app.settings.get('localCurrency');
    },
    viewingCurrency () {
      return getExchangeRate(this.displayCurrency) !== undefined ? this.displayCurrency : this.prices[0].currency;
    },
    totalTip () {
      let totalTip = "";
      let hasNonViewingCurrency = false;
      let hasNonPaymentCurrency = false;
      this.prices.forEach((priceObj, i) => {
        if (priceObj.currency !== this.viewingCurrency) {
          hasNonViewingCurrency = true;
        }
        if (priceObj.currency !== this.paymentCoin) {
          hasNonPaymentCurrency = true;
        }
      });

      if (hasNonViewingCurrency) {
        totalTip = this.ob.polyT('purchase.receipt.totalWarning1', { currency: this.viewingCurrency });
      } else if (hasNonPaymentCurrency) {
        if (this.paymentCoin) {
          totalTip = this.ob.polyT('purchase.receipt.totalWarning2', { currency: this.paymentCoin });
        } else {
          totalTip = this.ob.polyT('purchase.receipt.totalWarning2NoCoin');
        }
      }

      return totalTip;
    },

    shippingTotal() {
      const shippingPrice = this.options.totalShippingPrice;

      return convertAndFormatCurrency(shippingPrice.price ?? 0, shippingPrice.currency ?? this.viewingCurrency, this.viewingCurrency);
    },

    totalPrice() {
      const shippingPrice = this.options.totalShippingPrice;
      const shippingTotal = convertCurrency(shippingPrice.price ?? 0, shippingPrice.currency ?? this.viewingCurrency, this.viewingCurrency);

      let total = shippingTotal;
      this.updatedPrice.forEach(priceObj => total = total.plus(priceObj.subTotal));

      return total;
    },
    
    updatedPrice() {
      const updatedPrice = this.options.prices;
      updatedPrice.forEach((priceObj, i) => {
        // convert the prices here, to prevent rounding errors in the display
        const basePrice = convertCurrency(priceObj.price, priceObj.currency, this.viewingCurrency);

        const surcharge = convertCurrency(priceObj.vPrice, priceObj.currency, this.viewingCurrency);
        const optionalPrice = convertCurrency(priceObj.oPrice, priceObj.currency, this.viewingCurrency);

        const validQuantity = priceObj.quantity && !priceObj.quantity.isNaN() && priceObj.quantity.gt(0);

        priceObj.quantity = validQuantity ? priceObj.quantity : bigNumber(1);
        if (this.listing.isCrypto) {
          priceObj.quantity = validQuantity ? priceObj.quantity : bigNumber(0);
        }

        let itemTotal = basePrice.plus(surcharge).plus(optionalPrice);
        priceObj.preCouponPrice = itemTotal;
        this.options.coupons[i].forEach((coupon) => {
          if (coupon.percentDiscount) {
            itemTotal = itemTotal.minus(
              itemTotal.times(0.01).times(coupon.percentDiscount)
            );
          } else if (coupon.priceDiscount && !coupon.priceDiscount.isNaN()) {
            const convertPriceDiscount =
              convertCurrency(
                coupon.priceDiscount,
                priceObj.currency,
                this.viewingCurrency
              );
            itemTotal = itemTotal.minus(convertPriceDiscount);
          }
        });
        priceObj.subTotal = itemTotal.times(priceObj.quantity);

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
      return updatedPrice;
    },
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        paymentCoin: '',
        showTotalTip: true,
        ...options,
      };

      this.baseInit(opts);

      if (!this.model || !(this.model instanceof Order)) {
        throw new Error('Please provide an order model');
      }

      if (!this.listing || !(this.listing instanceof Listing)) {
        throw new Error('Please provide a listing model');
      }

      if (!this.prices) {
        throw new Error('Please provide the prices array');
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
