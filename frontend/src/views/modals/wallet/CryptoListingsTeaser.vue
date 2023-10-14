<template>
  <div>
    <div class="padLg txCtr">
      <CryptoTradingPair :options="ob.crypto.tradingPairOptions({
          className: 'cryptoTradingPairXL hideCodes rowMd',
          fromCur: 'BTC',
          toCur: 'ZEC',
        })" />
      <p class="tx4 rowMd">
        {{ ob.polyT('wallet.cryptoTeaser.bodyLine', { coinCount: ob.polyT('wallet.cryptoTeaser.coinCount', { count: 1500 }) }) }}
      </p>
      <div class="rowMd">
        <a class="btn js-createListing clrP clrBr clrSh3" @click="onClickCreateListing">
          {{ ob.polyT('wallet.cryptoTeaser.btnCreateListing') }}
        </a>
      </div>
      <template v-if="viewCryptoListingsUrl">
        <div @click="onClickViewCryptoListings">
          <a class="tx6 txU js-viewCryptoListings" :href="viewCryptoListingsUrl">
            {{ ob.polyT('wallet.cryptoTeaser.linkViewCryptoListings') }}
          </a>
        </div>
      </template>
    </div>

  </div>
</template>

<script>

export default {
  props: {
    viewCryptoListingsUrl: {
      type: String,
      default: ''
    }
  },
  methods: {
    onClickCreateListing() {
      this.$emit('createListing');
    },
    onClickViewCryptoListings() {
      this.$emit('viewCryptoListings');
    },
  }
}
</script>
<style lang="scss" scoped></style>
