<template>
  <div class="receipt flexColRows gutterVSm tx5b">
    <hr class="clrBr">
    <b>{{ polyT('purchase.receipt.summary') }}</b>

    <div v-for="(priceObj, i) in prices" :key="i">
      <div class="flexRow gutterHSm">
        <span class="flexExpand">
          {{ !isCrypto ?
        polyT('purchase.receipt.listing')
      :
        polyT('purchase.receipt.cryptoIconsAmountCombo', {
          icons: ob?.crypto?.tradingPair({
            className: 'cryptoTradingPairSm cryptoTradingPair',
            fromCur: listing.metadata.acceptedCurrencies[0],
            toCur: listing.item.cryptoListingCurrencyCode,
            truncateCurAfter: 5,
          }),
          cryptoIconsAmount:
            `<span class="cryptoQuantity">${polyT('purchase.receipt.cryptoIconsAmount',
              { amount: toStandardNotation(priceObj.quantity) })}</span>`,
        })
    }}
        </span>
        <div class="constrainedWidth">
          <div class="flexHRight">
            <b>
              {{ !isCrypto ? formatCurrency(priceObj.preCouponPrice, viewingCurrency) : formatCurrency(priceObj.subTotal.plus(priceObj.shippingTotal), viewingCurrency) }}
            </b>
          </div>
        </div>
      </div>
      <div v-for="(coupon, j) in coupons" :key="j">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">{{ polyT('purchase.receipt.coupon') }}</span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b> {{ coupon.percentDiscount ? `-${coupon.percentDiscount}%` 
            : (coupon.priceDiscount && !coupon.priceDiscount.isNaN() ? `-${convertAndFormatCurrency(coupon.priceDiscount, listingCurrency, viewingCurrency)}` : "")}}
              </b>
            </div>
          </div>
        </div>
      </div>

      <div v-if="listing.shippingOptions && listing.shippingOptions.length && priceObj.shippingPrice !== priceObj.additionalShippingPrice && priceObj.quantity > 1">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">
            {{ polyT('purchase.receipt.shipping') }}
          </span>
        </div>
        <div class="flexRow subShipping gutterHSm">
          <span class="flexExpand">
            {{ polyT('purchase.receipt.firstItem') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ formatCurrency(priceObj.shippingPrice, viewingCurrency) }}
              </b>
            </div>
          </div>
        </div>
        <div class="flexRow subShipping gutterHSm">
          <span class="flexExpand">
            {{ polyT('purchase.receipt.additionalItems') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ formatCurrency(priceObj.additionalShippingPrice, viewingCurrency) }}
              </b>
            </div>
          </div>
        </div>
      </div>
      <div v-else-if="listing.shippingOptions && listing.shippingOptions.length">
        <div class="flexRow gutterHSm">
          <span class="flexExpand">
            {{ polyT('purchase.receipt.shipping') }}
          </span>
          <div class="constrainedWidth">
            <div class="flexHRight">
              <b>
                {{ formatCurrency(priceObj.shippingPrice, viewingCurrency) }}
              </b>
            </div>
          </div>
        </div>
      </div>
      <hr class="clrBr">
      <div v-if="priceObj.quantity && priceObj.quantity.gt(0)">
        <div v-if="!isCrypto">
          <div class="flexRow gutterHSm">
            <span class="flexExpand">
              {{ polyT('purchase.receipt.subtotal', { quantity: toStandardNotation(priceObj.quantity) }) }}
            </span>
            <div class="constrainedWidth">
              <div class="flexHRight">
                <b>
                  {{ formatCurrency(priceObj.subTotal, viewingCurrency) }}
                </b>
              </div>
            </div>
          </div>
        </div>
        <div v-if="listing.shippingOptions && listing.shippingOptions.length && priceObj.shippingTotal">
          <div class="flexRow gutterHSm">
            <span class="flexExpand">
              {{ polyT('purchase.receipt.shippingTotal') }}
            </span>
            <div class="constrainedWidth">
              <div class="flexHRight">
                <b>
                  {{ formatCurrency(priceObj.shippingTotal, viewingCurrency) }}
                </b>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="flexRow">
        <span class="flexExpand">
          {{ polyT('purchase.receipt.total') }}
          <span class="toolTip clrTAlert" :data-tip="totalTip" v-show="showTotalTip"><span class="ion-alert-circled padSm"></span></span>
        </span>
        <div class="constrainedWidth">
          <div class="flexHRight">
            <b>
              {{ formatCurrency(priceObj.subTotal.plus(priceObj.shippingTotal), viewingCurrency) }}
            </b>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { convertCurrency, formatCurrency, convertAndFormatCurrency, getExchangeRate } from '../../../backbone/utils/currency';
import { toStandardNotation } from '../../../backbone/utils/number';
import bigNumber from 'bignumber.js';

const props = defineProps({
  order: Object,
  listing: Object,
  prices: Array,
  paymentCoin: String,
  coupons: Array,
  showTotalTip: Boolean,
})

const isCrypto = false;

const displayCurrency = window['app']?.settings.get('localCurrency');

const listingCurrency = props.listing.metadata.pricingCurrency.code;

const viewingCurrency = getExchangeRate(displayCurrency) !== undefined ? displayCurrency : listingCurrency;

let totalTip = "";
if (viewingCurrency !== listingCurrency) {
  totalTip = polyT('purchase.receipt.totalWarning1', { currency: viewingCurrency });
} else if (viewingCurrency !== props.paymentCoin) {
  if (props.paymentCoin) {
    totalTip = polyT('purchase.receipt.totalWarning2', { currency: props.paymentCoin });
  } else {
    totalTip = polyT('purchase.receipt.totalWarning2NoCoin');
  }
}

props.prices.forEach((priceObj, i) => {
  // convert the prices here, to prevent rounding errors in the display
  const basePrice = convertCurrency(priceObj.price, listingCurrency, viewingCurrency);

  priceObj.shippingPrice = convertCurrency(priceObj.sPrice, listingCurrency, viewingCurrency);

  priceObj.additionalShippingPrice = convertCurrency(priceObj.aPrice, listingCurrency, viewingCurrency);

  const surcharge = convertCurrency(priceObj.vPrice, listingCurrency, viewingCurrency);

  const validQuantity = priceObj.quantity && !priceObj.quantity.isNaN() && priceObj.quantity.gt(0);

  priceObj.quantity = validQuantity ? priceObj.quantity : bigNumber(1);
  if (isCrypto) {
    priceObj.quantity = validQuantity ? priceObj.quantity : bigNumber(0);
  }

  let itemTotal = basePrice.plus(surcharge);
  priceObj.preCouponPrice = itemTotal;
  props.coupons.forEach((coupon) => {
    if (coupon.percentDiscount) {
      itemTotal = itemTotal.minus(
        itemTotal.times(0.01).times(coupon.percentDiscount)
      );
    } else if (coupon.priceDiscount && !coupon.priceDiscount.isNaN()) {
      const convertPriceDiscount =
        convertCurrency(
          coupon.priceDiscount,
          listingCurrency,
          viewingCurrency
        );
      itemTotal = itemTotal.minus(convertPriceDiscount);
    }
  });
  priceObj.subTotal = itemTotal.times(priceObj.quantity);
  priceObj.shippingTotal = priceObj.shippingPrice.plus(priceObj.additionalShippingPrice.times(priceObj.quantity.minus(1)));
});

function polyT(key, options) {
  return window['app']?.polyglot.t(key, options);
}

</script>
<style lang="scss" scoped>
</style>
