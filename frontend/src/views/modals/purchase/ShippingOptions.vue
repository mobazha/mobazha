<template>
  <div class="shippingOptions">
    <template v-if="ob.validOptions.length">
      <div class="flexColRows boxList border borderStacked clrP clrBr">
        <template v-for="(service, i) in ob.validOptions" :key="i">
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
        </template>
      </div>
    </template>
    <template v-else>
      <div class="padGi flexCent">
        <h5>{{ ob.polyT('purchase.noShippableAddresses') }}</h5>
      </div>
    </template>

  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import Listing from '../../../../backbone/models/listing/Listing';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        validOptions: this.validOptions,
        selectedOption: this.selectedOption,
        displayCurrency: app.settings.get('localCurrency'),
      };
    }
  },
  methods: {
    loadData(options = {}) {
      this.baseInit(options);

      if (!this.model || !(this.model instanceof Listing)) {
        throw new Error('Please provide a listing model');
      }
    },
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
