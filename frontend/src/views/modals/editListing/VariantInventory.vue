<template>
  <div>
    <template v-if="!collection.length">
      <div class="rowMd">{{ ob.polyT('editListing.variantInventory.placeholderNeedMoreData') }}</div>
    </template>

    <template v-else>
      <div class="inventoryTableWrap rowSm">
        <table>
          <tr>
            <th class="clrBr">{{ ob.polyT('editListing.optionalFeatures.image') }}</th>
            <template v-for="(column, j) in columns" :key="j">
              <th class="clrBr">{{ column }}</th>
            </template>
            <th class="clrBr surcharge">{{ ob.polyT('editListing.variantInventory.surcharge') }}</th>
            <th class="clrBr totalPrice">{{ ob.polyT('editListing.variantInventory.totalPrice') }}</th>
            <th class="clrBr">{{ ob.polyT('editListing.variantInventory.sku') }}</th>
            <th class="clrBr quantityCol">{{ ob.polyT('editListing.variantInventory.quantity') }}</th>
            <th class="clrBr"></th>
          </tr>
          <template v-for="item in collection" :key="item.cid">
            <VariantInventoryItem
              ref="itemViews"
              :options="{
                basePrice: options.basePrice,
                listingCurrency: options.listingCurrency,
              }"
              :bb="
                function () {
                  return {
                    model: item,
                  };
                }
              "
              @removeClick="onRemoveClick"
            />
          </template>
        </table>
        <a class="clrBr clrP clrTEm" @click="onClickAddMissingSkus" v-if="collection.length < fullSkus.fullSkus.length">{{ ob.polyT('editListing.variantInventory.addMissingSku') }}</a>
      </div>
      <div class="clrT2 txSm helper">{{ ob.polyT('editListing.variantInventory.helperText') }}</div>
    </template>
  </div>
</template>

<script>
import _ from 'underscore';
import Sku from '../../../../backbone/models/listing/Sku';
import VariantInventoryItem from './VariantInventoryItem.vue';

export default {
  components: {
    VariantInventoryItem,
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
      };
    },

    variationOptions() {
      return this.optionsCl
        .toJSON()
        // only process options that have at least one variant
        .filter((option) => option.variation && option.variants && option.variants.length);
    },

    columns() {
      return this.variationOptions.map((option) => option.name);
    },

    // todo: good unit test candidate
    fullSkus() {
      const options = this.variationOptions;

      // ensure the Sku collection has the latest data from the UI
      this.setCollectionData();

      const existingSkus = [];
      const missingSkus = [];
      this.allPossibleCombos(options.map((option) => option.variants))
        .sort()
        .map((strCombo) => JSON.parse(`[${strCombo}]`))
        .forEach((combo) => {
          const choices = [];

          combo.forEach((comboIndex, index) => {
            choices.push(options[index].variants[comboIndex].name);
          });

          const selections = combo.map((val, idx) => ({
            option: options[idx].name,
            variant: options[idx].variants[val].name,
          }));

          let data = {
            choices,
            selections,
          };

          const id = this.buildIdFromSelections(selections);

          // If there is an existing sku for this selections, we'll
          // merge its data in
          const sku = this.collection.findWhere({ mappingId: id });

          if (sku) {
            data = {
              ...data,
              ...sku.toJSON(),
            };
            existingSkus.push(data);
          } else {
            // If no sku, we'll merge in a new Sku model so the model's
            // defaults get into the data
            data = {
              ...data,
              ...new Sku().toJSON(),
              mappingId: id,
            };
            missingSkus.push(data);
          }
        });

        const fullSkus = [...existingSkus, ...missingSkus];
        return {existingSkus, missingSkus, fullSkus}
    },
  },
  watch: {
    variationOptions: {
      handler() {
        if (!this.isSkusMatch()) {
          this.collection.reset(this.fullSkus.fullSkus);
        }
      },
      immediate: true
    },
  },
  methods: {
    loadData() {
      if (!this.collection) {
        throw new Error('Please provide a Skus collection.');
      }

      if (!this.optionsCl) {
        throw new Error('Please provide an options collection.');
      }

      // Give each Sku a mappingId which links it to the option it originated from
      // in a more robust way than relying on order which can change.
      if (this.optionsCl.length) {
        this.collection.forEach((sku) => {
          const selections = sku.get('selections');
          sku.set('mappingId', this.buildIdFromSelections(selections));
        });
      }

      this.collection.reset(this.fullSkus.existingSkus);
    },

    setCollectionData() {
      (this.$refs.itemViews ?? []).forEach((item) => item.setModelData());
    },

    onClickAddMissingSkus() {
      this.collection.push(this.fullSkus.missingSkus);
    },

    onRemoveClick(model) {
      this.collection.remove(model);
    },

    isSkusMatch() {
      const collection = this.collection.toJSON();
      const optionsCl = this.variationOptions;
      
      const options = {};
      optionsCl.forEach((option) => {
        options[option.name] = option.variants.map((variant) => variant.name);
      });

      for (let i = 0; i < collection.length; i += 1) {
        const selections = collection[i].selections;
        if (selections.length !== optionsCl.length) {
          return false;
        }

        for (let j = 0; j < selections.length; j += 1) {
          const selection = selections[j];
          if (!options[selection.option] || !options[selection.option].includes(selection.variant)) {
            return false;
          }
        }
      }

      return true;
    },

    // Inpsired by: http://stackoverflow.com/a/4331218/632806
    // TODO: would be nice to unit test this guy
    allPossibleCombos(arr) {
      let returnVal;

      if (!arr.length) {
        return [];
      }

      if (arr.length === 1) {
        returnVal = arr[0].map((val, index) => index);
      } else {
        const result = [];
        const allCasesOfRest = this.allPossibleCombos(arr.slice(1)); // recur with the rest of array
        for (let i = 0; i < allCasesOfRest.length; i += 1) {
          for (let j = 0; j < arr[0].length; j += 1) {
            result.push(`${j}, ${allCasesOfRest[i]}`);
          }
        }
        return result;
      }

      return returnVal;
    },

    buildIdFromSelections(selections, options = this.optionsCl) {
      if (!_.isArray(selections)) {
        throw new Error('Please provide a selections as an array.');
      }

      let id = '';

      selections.forEach((val) => {
        const option = options.find((opt) => opt.get('name') === val.option);

        if (option) {
          id += `${id.length ? '/' : ''}${option.id}-${val.variant}`;
        }
      });

      return id;
    },
  },
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
