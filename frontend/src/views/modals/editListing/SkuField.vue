<template>
  <div>
    <label for="editListingSku">{{ ob.polyT('editListing.sku') }}</label>
    <FormError v-if="ob.errors['productId']" :errors="ob.errors['productId']" />
    <input type="text" class="clrBr clrP clrSh2 marginTopAuto" :disabled="ob.variantsPresent" name="item.productID"
      id="editListingSku" :value="ob.productID" :placeholder="ob.polyT('editListing.placeholderSKU')"
      :maxlength="ob.max.productIdLength">
    <div class="clrT2 txSm helper" v-html='ob.variantsPresent ?
      ob.polyT("editListing.helperSKUWithVariants",
        { variantInventoryLink: `<a class="js-scrollToVariantInventory">${ob.polyT("editListing.variantInventoryLink")}</a>` }) :
      ob.polyT("editListing.helperSKU")'></div>
  </div>
</template>

<script>
import _ from 'underscore';

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
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        ...this._state,
        errors: this.model.validationError || {},
        max: {
          productIdLength: this.model.max.productIdLength,
        },
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (!this.model) {
        throw new Error('Please provide an item model.');
      }

      this._state = {
        variantsPresent: false,
        errors: [],
        ...options.initialState || {},
      };
    },

    setState (state, replace = false) {
      let newState;

      if (replace) {
        this._state = state;
      } else {
        newState = _.extend({}, this._state, state);
      }

      if (!_.isEqual(this._state, newState)) {
        this._state = newState;
        this.render();
      }

      return this;
    },
  }
}
</script>
<style lang="scss" scoped></style>
