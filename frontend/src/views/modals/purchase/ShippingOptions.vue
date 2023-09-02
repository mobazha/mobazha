<template>
  <div class="shippingOptions">
    <div v-if="validOptions.length">
      <div class="flexColRows boxList border borderStacked clrP clrBr">
        <div v-for="(service, i) in validOptions" :key="i">
          <div class="btnRadio width100">
            <input type="radio"
              @click="onSelectShippingOption(service)"
              :id="`option${i}`"
              :checked="isServiceSelected(service)"
              name="shippingOption">
            <label :for="`option${i}`" class="flex gutterH pad">
              <div>
                <div class="tx5 rowSm txB">{{ service.name }}: {{ service.service }}</div>
                <div class="tx5b clrT2 txUnb">
                  {{
                    service.estimatedDelivery
                      ? ob.polyT('purchase.serviceDetails', { price: getPrice(service), delivery: service.estimatedDelivery, })
                      : ''
                  }}
                </div>
              </div>
            </label>
          </div>
        </div>
      </div>
    </div>
    <div v-else>
      <div class="padGi flexCent">
        <h5>{{ ob.polyT('purchase.noShippableAddresses') }}</h5>
      </div>
    </div>

  </div>
</template>

<script>

export default {
  mixins: [],
  props: {
    listing: {
      type: Object,
      default: {},
    },
    validOptions: {
      type: Object,
      default: [],
    },
    selectedOption: {
      type: Object,
      default: {}
    }
  },
  mounted() {
  },
  methods: {
    isServiceSelected (service) {
      return this.selectedOption && this.selectedOption.name === service.name && this.selectedOption.service === service.service;
    },

    getPrice (service) {
      let price = '';

      try {
        price = this.ob.currencyMod.convertAndFormatCurrency(service.price, this.listing.metadata.pricingCurrency.code, this.listing.displayCurrency);
      } catch (e) {
        // pass
      }

      return price;
    },

    onSelectShippingOption (service) {
      this.$emit('shippingOptionSelected', { name: service.name, service: service.service })
    },
  }
}
</script>
<style lang="scss" scoped></style>
