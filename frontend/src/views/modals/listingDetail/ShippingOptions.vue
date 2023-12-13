<template>
  <template v-if="ob.shippingOptions.length">
    <template v-for="(option, tdi) in ob.shippingOptions" :key="option">
      <template v-if="option.get('type') !== 'LOCAL_PICKUP'">
        <ShippingOptionDetail class="tx5 clrBr borderBottom" v-show="option.get('services').length"
          :bb="() => {
            return {
              shippingOption: option,
            }
          }"
        />
      </template>
      <template v-else>
        <table class="shippingTable clrBr">
          <tr class="tx5 clrBr borderBottom">
            <th>{{ option.get('name') }}</th>
            <template v-if="option.get('type') === 'LOCAL_PICKUP'">
              <th colspan="3">{{ ob.polyT('listingDetail.localPickup') }}</th>
            </template>
          </tr>

          <!-- <template v-for="(service, si) in sortedService(option)" :key="service">
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
                      service.firstFreight,
                      option.currency,
                      ob.displayCurrency
                    )
                  }}
                </template>
              </td>
              <td>
                <template v-if="service.renewalUnitPrice && service.renewalUnitPrice.eq(0)">
                  <div class="clrE1 clrTOnEmph phraseBox floL">{{ ob.polyT('listingDetail.freeShippingBanner') }}</div>
                </template>

                <template v-else>
                  {{
                    ob.currencyMod.convertAndFormatCurrency(
                      service.renewalUnitPrice,
                      option.currency,
                      ob.displayCurrency
                    )
                  }}
                </template>
              </td>
            </tr>
          </template> -->
        </table>
      </template>
    </template>
    
  </template>

  <template v-else>
    <hr class="row">
    <div class="rowLg tx4 txCtr">
      {{ ob.polyT('listingDetail.noShippingFound') }}
    </div>
  </template>
</template>

<script>
import ShippingOptionDetail from '@/views/modals/settings/ShippingOptionDetail.vue';

export default {
  components: {
    ShippingOptionDetail,
  },
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
      return service.firstFreight && service.firstFreight.eq(0) ? true : false;
    }
  }
}
</script>
<style lang="scss" scoped></style>
