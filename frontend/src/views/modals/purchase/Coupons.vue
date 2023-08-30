<template>
  <div class="coupons">
    <div v-if="ob.codeResult && ob.codeResult.type && ob.codeResult.type !== 'valid'">
      <div class="txSm rowTn flex">
        <span class="clrTErr">
          <div v-if="ob.codeResult.code">
            {{ ob.polyT(`purchase.codeErrors.${ob.codeResult.type}`, { code: ob.codeResult.code }) }}
          </div>

          <div v-else>
            {{ ob.polyT('purchase.codeErrors.blank') }}
          </div>
        </span>
      </div>
    </div>
    <div v-for="(code, j) in ob.couponCodes" :key="j">
      <div class="txSm rowTn flexVCent gutterH">
        <span class="clrTEm">{{ ob.polyT('purchase.code', { code }) }}</span>
        <button class="btnTxtOnly " @click="clickRemove" :data-code="code">
          {{ ob.polyT('purchase.removeCode') }}
        </button>
      </div>
    </div>

  </div>
</template>

<script setup>
import $ from 'jquery';
import bigNumber from 'bignumber.js';
import multihashes from 'multihashes';
import loadTemplate from '../../../../backbone/utils/loadTemplate';
import { isValidNumber } from '../../../../backbone/utils/number';

const props = defineProps({
  phase: String,
  outdatedHash: String,
})

loadData(props);

render();

function loadData (options = {}) {
  super(options);
  this.options = options;

  if (!isValidNumber(options.listingPrice)) {
    throw new Error('Please provide a string based number as the price of the listing.');
  }

  this.couponCodes = [];
  this.couponHashes = [];
  this.listingPrice = options.listingPrice;
  this.totalDiscount = bigNumber(0);
  this.coupons = options.coupons;
  this.codeResult = {};
}

function sha256 (str) {
  // adapted from https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/digest
  const buffer = new TextEncoder('utf-8').encode(str);
  return crypto.subtle.digest('SHA-256', buffer).then(hash => hex(hash));
}

function hex (buffer) {
  // adapted from https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/digest
  const hexCodes = [];
  const view = new DataView(buffer);
  for (let i = 0; i < view.byteLength; i += 4) {
    const value = view.getUint32(i);
    const stringValue = value.toString(16);
    const padding = '00000000';
    const paddedValue = (padding + stringValue).slice(-padding.length);
    hexCodes.push(paddedValue);
  }

  return hexCodes.join('');
}

function addCode (code) {
  return sha256(code).then(hash => {
    const buf = new Buffer(hash, 'hex');
    const encoded = multihashes.encode(buf, 'sha2-256');
    const hashedCode = multihashes.toB58String(encoded);
    const coupon = findCoupon(hashedCode, code);
    const discount = couponDiscount(coupon);
    this.codeResult = { type: 'valid', code };

    if (coupon) {
      // don't add duplicate coupons
      if (this.couponCodes.indexOf(code) !== -1) {
        this.codeResult = { type: 'duplicate', code };
        // don't add if the total discount is more than the price of the listing
      } else if (this.totalDiscount.plus(discount).lt(this.listingPrice)) {
        this.totalDiscount = this.totalDiscount.plus(discount);
        this.couponCodes.push(code);
        this.couponHashes.push(hashedCode);
        this.trigger('changeCoupons', this.couponHashes, this.couponCodes);
      } else {
        this.codeResult = { type: 'excessive', code };
      }
    } else {
      this.codeResult = { type: 'invalid', code };
    }
    render();
    return this.codeResult;
  });
}

function findCoupon (hashedCode, code) {
  return this.coupons.findWhere({ hash: hashedCode }) ||
    this.coupons.findWhere({ discountCode: code });
}

function couponDiscount (coupon) {
  const percDis = coupon && coupon.get('percentDiscount') || 0;
  const pricDis = coupon && coupon.get('priceDiscount') || 0;
  return (this.listingPrice.times(percDis * 0.01).plus(pricDis));
}

function removeCode (code) {
  const index = this.couponCodes.indexOf(code);
  this.couponCodes.splice(index, 1);
  this.couponHashes.splice(index, 1);
  this.totalDiscount =
    this.totalDiscount.minus(
      couponDiscount(findCoupon('', code))
    );
  this.trigger('changeCoupons', this.couponHashes, this.couponCodes);
  this.codeResult = { type: 'valid', code };
  render();
}

function clickRemove (e) {
  removeCode($(e.target).attr('data-code'));
}

function render () {
  loadTemplate('modals/purchase/coupons.html', t => {
    this.$el.html(t({
      couponCodes: this.couponCodes,
      codeResult: this.codeResult,
    }));
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
