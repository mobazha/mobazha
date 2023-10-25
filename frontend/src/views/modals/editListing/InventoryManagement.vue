<template>
  <div>
    <h2 class="h4 clrT">{{ ob.polyT('editListing.sectionNames.inventoryManagement') }}</h2>
    <hr class="clrBr rowMd">
    <div class="flexRow gutterH rowSm">
      <div class="col6 simpleFlexCol">
        <select id="editInventoryManagementType" @change="onChangeManagementType" class="clrBr clrP clrSh2 marginTopAuto">
          <option value="DO_NOT_TRACK" :selected="ob.trackBy === 'DO_NOT_TRACK'">
            {{ ob.polyT('editListing.inventoryManagement.doNotTrackSelectOption') }}
          </option>
          <option value="TRACK" :selected="ob.trackBy !== 'DO_NOT_TRACK'">
            {{ ob.polyT('editListing.inventoryManagement.trackSelectOption') }}
          </option>
        </select>
      </div>
      <template v-if="ob.trackBy === 'TRACK_BY_FIXED'">
        <div class="col6 simpleFlexCol">
          <div>
            <FormError v-if="ob.errors['quantity']" :errors="ob.errors['quantity']" :class="margL" />
            <div class="flexVCent">
              <span class="margL margR">{{ ob.polyT('editListing.inventoryManagement.quantity') }}</span>
              <input type="text" class="clrBr clrP clrSh2 quantityInput" @change="onChangeQuantityInput"
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

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        trackBy: 'DO_NOT_TRACK', // DO_NOT_TRACK, TRACK_BY_FIXED, TRACK_BY_VARIANT
        errors: {},
      }
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
    $('#editInventoryManagementType').select2({
      // disables the search box
      minimumResultsForSearch: Infinity,
    });
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
    helperText () {
      const ob = this.ob;

      let helperText = ob.polyT('editListing.inventoryManagement.doNotTrackHelperText');

      if (ob.trackBy === 'TRACK_BY_FIXED') {
        helperText = ob.polyT('editListing.inventoryManagement.trackByFixedHelperText');
      } else if (ob.trackBy === 'TRACK_BY_VARIANT') {
        helperText = ob.polyT('editListing.inventoryManagement.trackByVariantsHelperText');
      }

      return helperText;
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this._state = {
        trackBy: 'DO_NOT_TRACK', // DO_NOT_TRACK, TRACK_BY_FIXED, TRACK_BY_VARIANT
        errors: {},
        ...options.initialState || {},
      };
    },

    onChangeQuantityInput (e) {
      this._state = {
        ...this._state,
        quantity: e.target.value,
      };
    },

    onChangeManagementType (e) {
      this.$emit('changeManagementType', { value: e.target.value });
    },

  }
}
</script>
<style lang="scss" scoped></style>
