<template>
  <section>
    <div class="contentBox pad clrP clrBr clrSh3 tx3">
      <form class="padSmKids padStack">
        <div class="flexVCent">
          <h2 class="h4 clrT flexExpand" :required="ob.listPosition === 1">
            {{ ob.polyT('settings.storeTab.shippingOptions.optionHeading', { listPosition: ob.listPosition }) }}
          </h2>
          <a class="clrBr clrP clrSh2 margRSm btn js-removeShippingOption" @click="onClickRemoveShippingOption">{{
            ob.polyT('settings.storeTab.shippingOptions.btnDeleteShippingOption')
          }}</a>
        </div>
        <hr class="clrBr rowMd" />
        <div class="flexRow">
          <label :for="`shipDestinationsSelect_${ob.cid}`" class="required">{{ ob.polyT('settings.storeTab.shippingOptions.shippingDestinations') }}</label>
          <div class="flexExpand">
            <div class="flexHRight flexVCent">
              <a class="js-clearAllShipDest tx6" @click="onClickClearShipDest">{{ ob.polyT('settings.storeTab.shippingOptions.clearAll') }}</a>
            </div>
          </div>
        </div>
        <FormError v-if="ob.errors['regions']" :errors="ob.errors['regions']" />
        <select
          :id="`shipDestinationsSelect_${ob.cid}`"
          ref="shipDestinationSelect"
          multiple
          class="clrBr clrP clrSh2"
          :placeholder="ob.polyT('settings.storeTab.shippingOptions.regionsPlaceholder')"
        ></select>
        <div class="flexRow gutterH">
          <div class="col6 simpleFlexCol">
            <label :for="`shipOptionTitle_${ob.cid}`" class="required">{{ ob.polyT('settings.storeTab.shippingOptions.nameLabel') }}</label>
            <FormError v-if="ob.errors['name']" :errors="ob.errors['name']" />
            <input
              type="text"
              class="clrBr clrP clrSh2 marginTopAuto"
              v-model="formData.name"
              :id="`shipOptionTitle_${ob.cid}`"
              :placeholder="ob.polyT('settings.storeTab.shippingOptions.namePlaceholder')"
            />
          </div>
          <div class="col4 simpleFlexCol">
            <label :for="`shipOptionType_${ob.cid}`" class="required">{{ ob.polyT('settings.storeTab.shippingOptions.typeLabel') }}</label>
            <FormError v-if="ob.errors['type']" :errors="ob.errors['type']" />
            <Select2
              :id="`shipOptionType_${ob.cid}`"
              :options="{ minimumResultsForSearch: Infinity }"
              @change="onChangeShippingType(val)"
              v-model="formData.type"
              class="clrBr clrP clrSh2 marginTopAuto"
            >
              <template v-for="(shippingType, j) in ob.shippingTypes" :key="j">
                <option :value="shippingType" :selected="formData.type === shippingType">
                  {{ ob.polyT(`settings.storeTab.shippingOptions.shippingTypes.${shippingType}`) }}
                </option>
              </template>
            </Select2>
          </div>
          <div class="col2 simpleFlexCol">
            <label :for="`currency_${ob.cid}`" class="required">{{ ob.polyT('settings.currency') }}</label>
            <FormError v-if="ob.errors['currency']" :errors="ob.errors['currency']" />
            <Select2 :id="`currency_${ob.cid}`" v-model="formData.currency" class="clrBr clrP clrSh2 marginTopAuto">
              <template v-for="(currency, j) in currencies" :key="j">
                <option :value="currency.code" :selected="currency.code.toUpperCase() === formData.currency.toUpperCase()">
                  {{ currency.code }}
                </option>
              </template>
            </Select2>
          </div>
        </div>
        <ShippingOptionDetail v-show="formData.type !== 'LOCAL_PICKUP' && model.get('services').length" :key="shippingOptionKey"
          :bb="() => {
            return {
              shippingOption: model,
            }
          }"
        />
        <div class="flexRow pad js-serviceSection" v-show="formData.type !== 'LOCAL_PICKUP'">
          <a class="clrBr clrP clrTEm js-btnAddService" @click="addExpressInfo">{{ ob.polyT('settings.storeTab.shippingOptions.services.addService') }}</a>
        </div>
      </form>
      <ShippingOptionModal ref="modal"
        :bb="() => {
          return {
            shippingOption: model,
          }
        }"
        @shippingOptionUpdated="shippingOptionKey += 1"
      />
    </div>
  </section>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import '../../../../backbone/utils/lib/selectize';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
import regions, { getTranslatedRegions, getIndexedRegions } from '../../../../backbone/data/regions';
import { getCurrenciesSortedByCode } from '../../../../backbone/data/currencies';
import ServiceMd from '../../../../backbone/models/settings/Service';

import ShippingOptionDetail from './ShippingOptionDetail.vue';
import ShippingOptionModal from './ShippingOptionModal.vue';

export default {
  components: {
    ShippingOptionDetail,
    ShippingOptionModal,
  },
  emits: ['click-remove'],
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      formData: {
        regions: [],
        name: '',
        type: '',
        currency: app.settings.get('localCurrency'),
      },
      currencies: getCurrenciesSortedByCode(),
      shippingOptionKey: 0,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.render();
  },
  watch: {
    'formData.name'(){
      this.model.set('name', this.formData.name);
    }
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        // Since multiple instances of this view will be rendered, any id's should
        // include the cid, so they're unique.
        cid: this.model.cid,
        listPosition: this.options.listPosition,
        shippingTypes: this.model.shippingTypes,
        errors: this.model.validationError || {},
        ...this.model.toJSON(),
      };
    },
  },
  methods: {
    initFormData() {
      const model = this.model.toJSON();
      this.formData = {
        regions: model.regions,
        name: model.name,
        type: model.type,
        currency: model.currency,
      };
    },
    loadData() {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.initFormData();

      // get regions
      this.selectCountryData = getTranslatedRegions().map((regionObj) => ({
        id: regionObj.id,
        text: regionObj.name,
        isRegion: true,
      }));

      // now, we'll add in the countries
      const selectCountries = getTranslatedCountries().map((countryObj) => ({
        id: countryObj.dataName,
        text: countryObj.name,
        isRegion: false,
      }));
      this.selectCountryData = this.selectCountryData.concat(selectCountries);

      this.services = this.model.get('services');
    },

    onClickRemoveShippingOption() {
      this.$emit('click-remove', this.model);
    },

    onClickAddService() {
      this.services.push(new ServiceMd());
    },

    onClickClearShipDest() {
      this.shipDestinationSelect[0].selectize.clear();
    },

    onChangeShippingType(val) {
    },

    getFormDataEx() {
      this.formData.regions = this.shipDestinationSelect[0].selectize.items;

      const formData = this.formData;
      const indexedRegions = getIndexedRegions();

      // Strip out any region elements from shipping destinations
      // drop down. The individual countries will remain.
      formData.regions = formData.regions.filter((region) => !indexedRegions[region]);

      return formData;
    },

    // Sets the model based on the current data in the UI.
    setModelData() {
      // set the data for our nested Services views
      if (this.formData.type === 'LOCAL_PICKUP') {
        this.model.set('services', []);
      }

      this.model.set(this.getFormDataEx());
    },

    onRemoveService(md) {
      this.services.remove(md);
    },

    /**
     * Returns a list of any regions that are fully represented in the provided
     * countries list.
     */
    representedRegions(countries = []) {
      if (!Array.isArray(countries)) {
        throw new Error('Please provide an array of country codes.');
      }

      const selectedRegions = [];

      regions.forEach((region) => {
        if (region.countries.every((elem) => countries.indexOf(elem) > -1)) {
          selectedRegions.push(region.id);
        }
      });

      return selectedRegions;
    },

    addExpressInfo() {
      this.$refs.modal.open();
    },

    render() {
      this.shipDestinationSelect = $(`#shipDestinationsSelect_${this.model.cid}`); //$(this.$refs.shipDestinationSelect);

      this.shipDestinationSelect.selectize({
        maxItems: null,
        valueField: 'id',
        searchField: ['text', 'id'],
        items: this.model.get('regions'),
        options: this.selectCountryData,
        render: {
          option: (data) => {
            const className = data.isRegion ? 'region' : '';
            return `<div class="${className}">${data.text}</div>`;
          },
          item: (data) => {
            const className = data.isRegion ? 'region' : '';
            return `<div class="${className}">${data.text}</div>`;
          },
        },
        onItemAdd: (value) => {
          const region = getIndexedRegions()[value];
          const { selectize } = this.shipDestinationSelect[0];

          if (region) {
            // If adding a region, we'll add in all the countries for that region.
            // selectize.removeItem(value);
            selectize.addItems(region.countries, true);
          } else {
            // Adding a country may cause a region or regions to be represented.
            // We'll add in any full regions so they're not selectable as options.
            // CSS will hide the tag.
            selectize.addItems(this.representedRegions(selectize.items), true);
          }
        },
        onItemRemove: (value) => {
          const isRegion = !!getIndexedRegions()[value];
          const { selectize } = this.shipDestinationSelect[0];
          const representedRegions = this.representedRegions(selectize.items);

          if (!isRegion) {
            // Adding a country may cause a regions or regions to be represented.
            // We'll add in any full regions so they're not selectable as options.
            // CSS will hide the tag.
            selectize.items.forEach((item) => {
              const isItemRegion = getIndexedRegions()[item];
              if (isItemRegion && !representedRegions.includes(item)) {
                selectize.removeItem(item, true);
              }
            });
          }
        },
      });

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
