<template>
  <div>
    <template v-if="!collection.length">
      <a class="btn clrP clrBr clrSh2" @click="onClickAddOptionalFeature">{{ ob.polyT('editListing.optionalFeatures.btnAddOptionalFeature') }}</a>
    </template>

    <template v-else>
      <div class="mb2">
        <table>
          <tr>
            <th class="clrBr"><label class="required">{{ ob.polyT('editListing.optionalFeatures.name') }}</label></th>
            <th class="clrBr surcharge">{{ ob.polyT('editListing.optionalFeatures.surcharge') }}</th>
            <th class="clrBr">{{ ob.polyT('editListing.optionalFeatures.sku') }}</th>
            <th class="clrBr">{{ ob.polyT('editListing.optionalFeatures.image') }}</th>
            <th class="clrBr"></th>
          </tr>
          <template v-for="model in collection" :key="model.cid">
            <OptionalFeatureItem
              ref="itemViews"
              :bb="
                function () {
                  return {
                    model,
                  };
                }
              "
              @removeClick="onRemoveClick"
            />
          </template>
        </table>
      </div>
      <a class="clrBr clrP clrTEm" v-show="collection.length > 0 && collection.length < ob.maxOptionalFeatureCount" @click="onClickAddOptionalFeature">{{ ob.polyT('editListing.optionalFeatures.addOptionalFeature') }}</a>
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
  created() {
    this.loadData(this.options);
  },
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        maxOptionalFeatureCount: this.options.maxOptionalFeatureCount,
      };
    }
  },
  methods: {
    loadData(options = {}) {
      if (!this.collection) {
        throw new Error('Please provide an VariantOptions collection.');
      }

      if (typeof options.maxOptionalFeatureCount === 'undefined') {
        throw new Error('Please provide the maximum optional feature count.');
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
    },

    setCollectionData() {
      this.$nextTick(() => {
        (this.$refs.itemViews ?? []).forEach((item) => item.setModelData());
      });
    },

    onRemoveClick(model) {
      this.collection.remove(model);
    },
  }
};
</script>
<style lang="scss" scoped>
.surcharge {
  width: 120px;
}
</style>
