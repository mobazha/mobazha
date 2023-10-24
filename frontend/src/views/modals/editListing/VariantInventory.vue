<template>
  <div>
    <template v-if="!ob.inventory.length">
      <div class="rowMd">{{ ob.polyT('editListing.variantInventory.placeholderNeedMoreData') }}</div>
    </template>

    <template v-else>
      <div class="inventoryTableWrap rowSm">
        <table>
          <tr>
            <template v-for="(column, j) in ob.columns" :key="j">
              <th class="clrBr">{{ column }}</th>
            </template>
            <th class="clrBr">{{ ob.polyT('editListing.variantInventory.surcharge') }}</th>
            <th class="clrBr">{{ ob.polyT('editListing.variantInventory.totalPrice') }}</th>
            <th class="clrBr">{{ ob.polyT('editListing.variantInventory.sku') }}</th>
            <th class="clrBr quantityCol">{{ ob.polyT('editListing.variantInventory.quantity') }}</th>
          </tr>
        </table>
      </div>
      <div class="clrT2 txSm helper">{{ ob.polyT('editListing.variantInventory.helperText') }}</div>
    </template>

  </div>
</template>

<script>
import _ from 'underscore';
import Sku from '../../../../backbone/models/listing/Sku';
import VariantInventoryItem from './VariantInventoryItem';


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
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      const inventoryData = this.buildInventoryData();
      this.collection.set(inventoryData.inventory);

      return {
        ...this.templateHelpers,
        columns: inventoryData.columns,
        inventory: this.collection.toJSON(),
        getPrice: this.options.getPrice,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.collection) {
        throw new Error('Please provide a Skus collection.');
      }

      if (!options.optionsCl) {
        throw new Error('Please provide an options collection.');
      }

      if (typeof options.getPrice !== 'function') {
        throw new Error('Please provide a getPrice function that returns the product price.');
      }

      if (typeof options.getCurrency !== 'function') {
        throw new Error('Please provide a function for me to obtain the current currency.');
      }

      this.baseInit(options);

      this.optionsCl = options.optionsCl;
      this.itemViews = [];

      // Give each Sku a mappingId which links it to the option it originated from
      // in a more robust way than relying on order which can change.
      if (this.optionsCl.length) {
        this.collection.forEach((sku) => {
          const selections = sku.get('selections');
          sku.set('mappingId', this.buildIdFromSelections(selections));
        });
      }

      this.listenTo(this.optionsCl, 'change:name', () => this.render());
      this.listenTo(this.optionsCl, 'update', (cl, opts) => {
        if (opts.changes.added.length) {
          this.bindOptionVariantsUpdate(opts.changes.added);
        }

        this.render();
      });
      this.bindOptionVariantsUpdate(this.optionsCl.models);
    },

    bindOptionVariantsUpdate (options = []) {
      options.forEach((option) => {
        this.listenTo(option.get('variants'), 'update', () => this.render());
      });
    },

    setCollectionData () {
      this.itemViews.forEach((item) => item.setModelData());
    },

    get $formFields () {
      if (!this._$formFields) {
        this._$formFields = $('select[name], input[name], textarea[name]');
      }
      return this._$formFields;
    },

    // Inpsired by: http://stackoverflow.com/a/4331218/632806
    // TODO: would be nice to unit test this guy
    allPossibleCombos (arr) {
      let returnVal;

      if (!arr.length) {
        return [];
      }

      if (arr.length === 1) {
        returnVal = arr[0].map((val, index) => (index));
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

    buildIdFromSelections (selections, options = this.optionsCl) {
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

    // todo: good unit test candidate
    buildInventoryData () {
      const options = this.optionsCl.toJSON()
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
              ...((new Sku()).toJSON()),
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

    render () {
      this.itemViews.forEach((item) => item.remove());
      this.itemViews = [];
      const itemsFrag = document.createDocumentFragment();

      this.collection.forEach((item) => {
        const view = this.createChild(VariantInventoryItem, {
          model: item,
          getPrice: this.options.getPrice,
          getCurrency: this.options.getCurrency,
        });

        this.itemViews.push(view);
        view.render().$el.appendTo(itemsFrag);
      });

      $('table').append(itemsFrag);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
