<template>
  <div class="shippingOptions">
    <template v-if="ob.validOptions.length">
      <div class="flexColRows boxList border borderStacked clrP clrBr" :key="ob.displayCurrency">
        <template v-for="(service, i) in ob.validOptions" :key="service.name">
          <div class="btnRadio width100">
            <input type="radio" @click="onSelectShippingOption(service)" :id="`option${i}`" :checked="isServiceSelected(service)" name="shippingOption">
            <label :for="`option${i}`" class="flex gutterH pad">
              <div>
                <div class="tx5 rowSm txB">{{ service.name }}: {{ service.service }}</div>
                <div class="tx5b clrT2 txUnb">
                  {{
                    service.estimatedDelivery ? ob.polyT('purchase.serviceDetails', { price: getPrice(service), delivery: service.estimatedDelivery, }) : ''
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
import app from '../../../../backbone/app';

export default {
  props: {
    options: {
      type: Object,
      default: {
        getTotalShippingPrice: undefined,
      },
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
        ...this.options,
        displayCurrency: app.settings.get('localCurrency'),
      };
    }
  },
  methods: {
    isServiceSelected (service) {
      return this.options.selectedOption && this.options.selectedOption.name === service.name && this.options.selectedOption.service === service.service;
    },

    getPrice (service) {
      const ob = this.ob;

      let shippingTotal = {};
      if (this.options.getTotalShippingPrice) {
        shippingTotal = this.options.getTotalShippingPrice(service.name, service.service);
      }

      let price = '';
      try {
        price = ob.currencyMod.convertAndFormatCurrency(shippingTotal.price, shippingTotal.currency, ob.displayCurrency);
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
