<template>
  <div>
    <div class="flexRow gutterH">
      <div class="col4">
        <label class="required">{{ ob.polyT('editListing.variants.titleLabel') }}</label>
      </div>
      <div class="col6">
        <label class="required">{{ ob.polyT('editListing.variants.choicesLabel') }}</label>
      </div>
      <div class="col2">
        <label class="required">{{ ob.polyT('editListing.variants.variationLabel') }}</label>
      </div>
    </div>
    <div class="js-variantsWrap padKids padStack padTop0">
      <template v-for="model in collection" :key="model.cid">
        <Variant ref="_variantViews" :options="variantViewOptions(model)" :bb="function() {
          return {
            model,
          }
        }"
        @removeClick="onRemoveClick"
        @update="this.$emit('update')" />
      </template>
    </div>
    <a class="clrBr clrP clrTEm btnAddVariant js-btnAddVariant" v-show="ob.variants.length < ob.maxVariantCount"
      @click="onClickAddVariant">{{ ob.polyT('editListing.variants.addVariant') }}</a>
  </div>
</template>

<script>
import VariantOption from '../../../../backbone/models/listing/VariantOption';
import Variant from './Variant.vue';

// There are some terminology mismatches between the UI and server. When the UI uses the
// term 'variant', it maps to an 'options' list in the listing API.

export default {
  components: {
    Variant,
  },
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
        variants: this.collection.toJSON(),
        maxVariantCount: this.options.maxVariantCount,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.collection) {
        throw new Error('Please provide an VariantOptions collection.');
      }

      if (typeof options.maxVariantCount === 'undefined') {
        throw new Error('Please provide the maximum variant count.');
      }

      // Certain variant validations are not possible to do purely in the model and
      // need to be done by a parent model. In that case higher-level errors can be passed
      // in the following format:
      // options.errors = {
      //   options[<model.cid>].<fieldName> = ['error1', 'error2', ...],
      //   options[<model.cid>].<fieldName2> = ['error1', 'error2', ...],
      //   options[<model2.cid>].<fieldName> = ['error1', 'error2', ...]
      // }
    },

    onClickAddVariant () {
      this.collection.add(new VariantOption());

      this.$nextTick(() => {
        if (this.collection.length) (this.$refs._variantViews[this.collection.length - 1]).setFocus();
      });
    },

    setCollectionData () {
      (this.$refs._variantViews ?? []).forEach((variant) => variant.setModelData());
    },

    setModelData (index) {
      if (typeof index !== 'number') {
        throw new Error('Please provide a numeric index.');
      }

      const view = this.$refs._variantViews[index];
      if (view) view.setModelData();
    },

    variantViewOptions(model) {
      const errors = {};

      if (this.options.errors) {
        Object.keys(this.options.errors)
          .forEach(errKey => {
            if (errKey.startsWith(`options[${model.cid}]`)) {
              errors[errKey.slice(errKey.indexOf('.') + 1)] = this.options.errors[errKey];
            }
          });
      }

      return { errors };
    },

    onRemoveClick(model) {
      this.collection.remove(model);
    },
  }
}
</script>
<style lang="scss" scoped></style>
