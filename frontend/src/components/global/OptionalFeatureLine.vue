<template>
  <div class="cell mt2" v-if="optionalFeatures?.length">
    <h5>{{ ob.polyT('editListing.sectionNames.optionalFeatures') }}</h5>
    <template v-for="(optionalFeature, j) in optionalFeatures">
      <div class="cell-item">
        {{ `${j+1}.` }}
        <el-image
          style="width: 16px; height: 16px"
          v-if="optionalFeature.images?.length"
          :src="ob.getServerUrl(`ob/image/${optionalFeature.images[0].small}`)"
          fit="cover"
          :preview-src-list="[ob.getServerUrl(`ob/image/${ob.isHiRez() ? optionalFeature.images[0].large : optionalFeature.images[0].medium}`)]"
        />
        {{ `${optionalFeature.name}, ${ob.currencyMod.convertAndFormatCurrency(optionalFeature.surcharge, pricingCurrency, displayCurrency)}${optionalFeature.skuID ? `, SKU: ${optionalFeature.skuID}` : ''}` }}
      </div>
    </template>
  </div>
</template>
<script>
export default {
  props: {
    optionalFeatures: {
      type: Array,
      default: [],
	  },
    pricingCurrency: {
      type: String,
      default: 'USD',
    },
    displayCurrency: {
      type: String,
      default: 'USD',
    }
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
      };
    },
  },
  methods: {
  }
}
</script>
<style lang="scss" scoped>

</style>
  