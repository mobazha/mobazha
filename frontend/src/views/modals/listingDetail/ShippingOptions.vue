<template>
  <div>
    <template v-if="ob.templateData.length">
      <table class="shippingTable clrBr">
        <template v-for="(option, tdi) in ob.templateData" :key="option">
          <tr class="tx5 clrBr borderBottom">
            <th>{{ option.name }}</th>
            <template v-if="option.type === 'LOCAL_PICKUP'">
              <th colspan="3">{{ ob.polyT('listingDetail.localPickup') }}</th>
            </template>

            <template v-else>
              <th>{{ ob.polyT('listingDetail.deliveryTime') }}</th>
              <th>{{ ob.polyT('listingDetail.priceFirst') }}</th>
              <th>{{ ob.polyT('listingDetail.priceSecond') }}</th>
            </template>
          </tr>

          <template v-for="(service, si) in sortedService(option)" :key="service">
            <tr :class="`${fShp(service) ? 'txB' : ''} ${si === option.services.length - 1 ? 'lastRow' : ''}`">
              <td>{{ service.name }}</td>
              <td>{{ service.estimatedDelivery }}</td>
              <td>
                <template v-if="fShp(service)">
                  <div class="clrE1 clrTOnEmph phraseBox floL">{{ ob.polyT('listingDetail.freeShippingBanner') }}</div>
                </template>

                <template v-else>
                  {{
                    ob.currencyMod.convertAndFormatCurrency(
                      service.price,
                      ob.pricingCurrency,
                      ob.displayCurrency
                    )
                  }}
                </template>
              </td>
              <td>
                <template v-if="service.additionalItemPrice && service.additionalItemPrice.eq(0)">
                  <div class="clrE1 clrTOnEmph phraseBox floL">{{ ob.polyT('listingDetail.freeShippingBanner') }}</div>
                </template>

                <template v-else>
                  {{
                    ob.currencyMod.convertAndFormatCurrency(
                      service.additionalItemPrice,
                      ob.pricingCurrency,
                      ob.displayCurrency
                    )
                  }}
                </template>
              </td>
            </tr>
          </template>
        </template>
      </table>
    </template>

    <template v-else>
      <hr class="row">
      <div class="rowLg tx4 txCtr">
        {{ ob.polyT('listingDetail.noShippingFound') }}
      </div>
    </template>

  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.options,
      }
    },
  },
  methods: {
    sortedService (option) {
      return option.services
        .sort((a, b) => {
          let sorter = 0;

          try {
            sorter = a.price.minus(b.price);
          } catch (e) {
            // pass
          }

          return sorter;
        })
    },
    fShp (service) {
      return service.price && service.price.eq(0) ? true : false;
    }
  }
}
</script>
<style lang="scss" scoped></style>
