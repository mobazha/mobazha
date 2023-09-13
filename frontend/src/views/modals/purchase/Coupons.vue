<template>
  <div class="coupons">
    <BaseModal>
      <template v-slot:component>
        <div v-if="codeResult && codeResult.type && codeResult.type !== 'valid'">
          <div class="txSm rowTn flex">
            <span class="clrTErr">
              <div v-if="codeResult.code">
                {{ ob.polyT(`purchase.codeErrors.${codeResult.type}`, { code: codeResult.code }) }}
              </div>

              <div v-else>
                {{ ob.polyT('purchase.codeErrors.blank') }}
              </div>
            </span>
          </div>
        </div>
        <div v-for="(code, j) in couponCodes" :key="j">
          <div class="txSm rowTn flexVCent gutterH">
            <span class="clrTEm">{{ ob.polyT('purchase.code', { code }) }}</span>
            <button class="btnTxtOnly " @click="removeCode(code)">
              {{ ob.polyT('purchase.removeCode') }}
            </button>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import bigNumber from 'bignumber.js';
import multihashes from 'multihashes';
import { isValidNumber } from '../../../../backbone/utils/number';


export default {
  mixins: [],
  props: {
    cart: Object,
  },
  data () {
    return {
      ob: {},
    };
  },
  mounted () {
    loadData(props);
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
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
    },

    sha256 (str) {
      // adapted from https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/digest
      const buffer = new TextEncoder('utf-8').encode(str);
      return crypto.subtle.digest('SHA-256', buffer).then(hash => this.hex(hash));
    },

    hex (buffer) {
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
    },

    addCode (code) {
      return this.sha256(code).then(hash => {
        const buf = new Buffer(hash, 'hex');
        const encoded = multihashes.encode(buf, 'sha2-256');
        const hashedCode = multihashes.toB58String(encoded);
        const coupon = this.findCoupon(hashedCode, code);
        const discount = this.couponDiscount(coupon);
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
        this.render();
        return this.codeResult;
      });
    },

    findCoupon (hashedCode, code) {
      return this.coupons.findWhere({ hash: hashedCode }) ||
        this.coupons.findWhere({ discountCode: code });
    },

    couponDiscount (coupon) {
      const percDis = coupon && coupon.get('percentDiscount') || 0;
      const pricDis = coupon && coupon.get('priceDiscount') || 0;
      return (this.listingPrice.times(percDis * 0.01).plus(pricDis));
    },

    removeCode (code) {
      const index = this.couponCodes.indexOf(code);
      this.couponCodes.splice(index, 1);
      this.couponHashes.splice(index, 1);
      this.totalDiscount =
        this.totalDiscount.minus(
          this.couponDiscount(this.findCoupon('', code))
        );
      this.trigger('changeCoupons', this.couponHashes, this.couponCodes);
      this.codeResult = { type: 'valid', code };
      this.render();
    },
  }
}
</script>
<style lang="scss" scoped></style>
