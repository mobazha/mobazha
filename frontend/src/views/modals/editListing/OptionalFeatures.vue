<template>
  <div>
    <template v-if="!collection.length">
      <a class="btn clrP clrBr clrSh2 addFirstVariant" @click="add">{{ ob.polyT('editListing.optionalFeatures.btnAddOptionalFeature') }}</a>
    </template>

    <template v-else>
      <div class="inventoryTableWrap mb2">
        <table>
          <tr>
            <th class="clrBr">Name</th>
            <th class="clrBr surcharge">{{ ob.polyT('editListing.variantInventory.surcharge') }}</th>
            <th class="clrBr">{{ ob.polyT('editListing.variantInventory.sku') }}</th>
            <th class="clrBr">Image</th>
            <th class="clrBr">Operate</th>
          </tr>
          <template v-for="item in collection" :key="item.cid">
            <OptionalFeatureItem
              ref="itemViews"
              :bb="
                function () {
                  return {
                    model: item,
                  };
                }
              "
            />
          </template>
        </table>
      </div>
      <a class="clrBr clrP clrTEm" v-show="collection.length > 0" @click="onClickAddVariant">{{ ob.polyT('editListing.optionalFeatures.addOptionalFeature') }}</a>
    </template>
  </div>
</template>

<script>
import _ from 'underscore';
import OptionalFeatureItem from './OptionalFeature.vue';
import OptionalFeature from '../../../../backbone/models/listing/OptionalFeature.js';

export default {
  components: {
    OptionalFeatureItem,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {};
  },
  created() {
    this.loadData(this.options);
  },
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        variants: this.collection.toJSON(),
        maxVariantCount: this.options.maxVariantCount,
      };
    }
  },
  methods: {
    loadData(options = {}) {
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

    onClickAddOptionalFeature() {
      this.collection.add(new OptionalFeature());

      this.$nextTick(() => {
        if (this.collection.length) (this.$refs.itemViews[this.collection.length - 1]).setFocus();
      });
    },

    setCollectionData() {
      this.$nextTick(() => {
        (this.$refs.itemViews ?? []).forEach((item) => item.setModelData());
      });
    },

    setModelData(index) {
      if (typeof index !== 'number') {
        throw new Error('Please provide a numeric index.');
      }

      const view = this.$refs.itemViews[index];
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
};
</script>
<style lang="scss" scoped>
.surcharge {
  width: 100px;
}
.totalPrice {
  min-width: 120px !important;
}
</style>
