<template>
  <div>
    <template v-if="!collection.length">
      <a class="btn clrP clrBr clrSh2 addFirstVariant" @click="add">Add Optional Features</a>
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
            <OptionalFeaturesItem
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
      <a class="clrBr clrP clrTEm" v-show="collection.length > 0" @click="onClickAddVariant">Add Optional Features</a>
    </template>
  </div>
</template>

<script>
import _ from 'underscore';
import Sku from '../../../../backbone/models/listing/Sku';
import OptionalFeaturesItem from './OptionalFeaturesItem.vue';

export default {
  components: {
    OptionalFeaturesItem,
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
      const inventoryData = this.inventoryData;
      this.collection.set(inventoryData.inventory);

      return {
        ...this.templateHelpers,
        columns: inventoryData.columns,
      };
    },
    // todo: good unit test candidate
    inventoryData() {
      const options = this.optionsCl
        .toJSON()
        // only process options that have at least one variant
        .filter((option) => option.variants && option.variants.length);

      const columns = options.map((option) => option.name);
      const inventoryData = [];

      // ensure the Sku collection has the latest data from the UI
      this.setCollectionData();

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
          } else {
            // If no sku, we'll merge in a new Sku model so the model's
            // defaults get into the data
            data = {
              ...data,
              ...new Sku().toJSON(),
              mappingId: id,
            };
          }

          inventoryData.push(data);
        });

      return {
        columns,
        inventory: inventoryData,
      };
    },
  },
  methods: {
    add() {},
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
    },

    setCollectionData() {
      (this.$refs.itemViews ?? []).forEach((item) => item.setModelData());
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
