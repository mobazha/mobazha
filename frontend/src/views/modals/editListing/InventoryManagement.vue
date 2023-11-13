<template>
  <div>
    <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.inventoryManagement') }}</h2>
    <hr class="clrBr rowMd">
    <div class="flexRow gutterH rowSm">
      <div class="col6 simpleFlexCol">
        <Select2 id="editInventoryManagementType" v-model="trackBy" @change="onChangeManagementType" class="clrBr clrP clrSh2 marginTopAuto" :options="{ minimumResultsForSearch: Infinity, }">
          <option value="DO_NOT_TRACK" :selected="trackBy === 'DO_NOT_TRACK'">
            {{ ob.polyT('editListing.inventoryManagement.doNotTrackSelectOption') }}
          </option>
          <option value="TRACK" :selected="trackBy !== 'DO_NOT_TRACK'">
            {{ ob.polyT('editListing.inventoryManagement.trackSelectOption') }}
          </option>
        </Select2>
      </div>
      <template v-if="options.trackBy === 'TRACK_BY_FIXED'">
        <div class="col6 simpleFlexCol">
          <div>
            <FormError v-if="ob.errors['quantity']" :errors="ob.errors['quantity']" :class="margL" />
            <div class="flexVCent">
              <span class="margL margR">{{ ob.polyT('editListing.inventoryManagement.quantity') }}</span>
              <input type="number" class="clrBr clrP clrSh2 quantityInput" @change="onChangeQuantityInput"
                name="item.quantity" :value="ob.quantity < 0 ? '' : ob.quantity" placeholder="0"
                data-var-type="bignumber">
            </div>
          </div>
        </div>
      </template>
    </div>

    <div class="clrT2 txSm helper">{{ helperText }}</div>
  </div>
</template>

<script>
import _ from 'underscore';
import bigNumber from 'bignumber.js';

export default {
  props: {
    options: {
      type: Object,
      default: {
        trackBy: 'DO_NOT_TRACK', // DO_NOT_TRACK, TRACK_BY_FIXED, TRACK_BY_VARIANT
        quantity: 0,
        errors: {},
      },
    },
  },
  data () {
    return {
      trackBy: 'DO_NOT_TRACK',
    };
  },
  created () {
    this.trackBy = (this.options.trackBy === 'DO_NOT_TRACK') ? 'DO_NOT_TRACK' : 'TRACK';
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.options,
      };
    },
    helperText () {
      const ob = this.ob;

      let helperText = ob.polyT('editListing.inventoryManagement.doNotTrackHelperText');

      if (this.options.trackBy === 'TRACK_BY_FIXED') {
        helperText = ob.polyT('editListing.inventoryManagement.trackByFixedHelperText');
      } else if (this.options.trackBy === 'TRACK_BY_VARIANT') {
        helperText = ob.polyT('editListing.inventoryManagement.trackByVariantsHelperText');
      }

      return helperText;
    },
  },
  methods: {
    onChangeQuantityInput (e) {
      if (!_.isEmpty(e.target.value)) {
        this.$emit('changeInventoryQuantity', bigNumber(e.target.value));
      }
    },

    onChangeManagementType (e) {
      this.$emit('changeManagementType', e.target.value);
    },
  }
}
</script>
<style lang="scss" scoped></style>
