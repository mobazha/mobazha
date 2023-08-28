<template>
  <div>

    <div class="flexColRows boxList border borderStacked clrP clrBr" v-if="ob.validOptions.length">

      <div v-for="(service, i) in ob.validOptions" :key="i">
        <div class="btnRadio width100">
          <input type="radio"
            class="js-shippingOption"
            :id="`option${i}`"
            :checked="isServiceSelected(service)"
            :data-name="service.name"
            :data-service="service.service"
            name="shippingOption">
          <label :for="`option${i}`" class="flex gutterH pad">
            <div>
              <div class="tx5 rowSm txB">{{ service.name }}: {{ service.service }}</div>
              <div class="tx5b clrT2 txUnb">
                {{ 
              service.estimatedDelivery ? ob.polyT('purchase.serviceDetails', {
                price: getPrice(service),
                delivery: service.estimatedDelivery }) : ''
              }}
              </div>
            </div>
          </label>
        </div>
      </div>
    </div>

    <div class="padGi flexCent" v-else>
      <h5>{{ ob.polyT('purchase.noShippableAddresses') }}</h5>
    </div>

  </div>
</template>

<script setup>

function isServiceSelected (service) {
  return ob.selectedOption && ob.selectedOption.name === service.name && ob.selectedOption.service === service.service;
}

function getPrice (service) {
  let price = '';

  try {
    price = ob.currencyMod.convertAndFormatCurrency(service.price, ob.metadata.pricingCurrency.code, ob.displayCurrency);
  } catch (e) {
    // pass
  }

  return price;
}

</script>
<style lang="scss" scoped>
</style>