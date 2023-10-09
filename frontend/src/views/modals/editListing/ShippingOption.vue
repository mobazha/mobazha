<template>
  <section>
    <div class="contentBox pad clrP clrBr clrSh3 tx3">
      <form class="padSmKids padStack">
        <div class="flexVCent">
          <h2 class="h4 clrT flexExpand" :required="ob.listPosition === 1">{{ ob.polyT('editListing.shippingOptions.optionHeading', { listPosition: ob.listPosition }) }}</h2>
          <a class="clrBr clrP clrSh2 margRSm btn js-removeShippingOption">{{ ob.polyT('editListing.shippingOptions.btnDeleteShippingOption') }}</a>
        </div>
        <hr class="clrBr rowMd" />
        <div class="flexRow">
          <label :for="`shipDestinationsSelect_${ob.cid}`" class="required">{{ ob.polyT('editListing.shippingOptions.shippingDestinations') }}</label>
          <div class="flexExpand">
            <div class="flexHRight flexVCent">
              <a class="js-clearAllShipDest tx6">{{ ob.polyT('editListing.shippingOptions.clearAll') }}</a>
            </div>
          </div>
        </div>
        <FormError v-if="ob.errors['regions']" :errors="ob.errors['regions']" />
        <select :id="`shipDestinationsSelect_${ob.cid}`" multiple name="regions" class="clrBr clrP clrSh2" :placeholder="ob.polyT('editListing.shippingOptions.regionsPlaceholder')"></select>
        <div class="flexRow gutterH">
          <div class="col6 simpleFlexCol">
            <label :for="`shipOptionTitle_${ob.cid}`" class="required">{{ ob.polyT('editListing.shippingOptions.nameLabel') }}</label>
            <FormError v-if="ob.errors['name']" :errors="ob.errors['name']" />
            <input type="text" class="clrBr clrP clrSh2 marginTopAuto" name="name" :id="`shipOptionTitle_${ob.cid}`" :value="ob.name" :placeholder="ob.polyT('editListing.shippingOptions.namePlaceholder')">
          </div>
          <div class="col6 simpleFlexCol">
            <label :for="`shipOptionType_${ob.cid}`" class="required">{{ ob.polyT('editListing.shippingOptions.typeLabel') }}</label>
            <FormError v-if="ob.errors['type']" :errors="ob.errors['type']" />
            <select :id="`shipOptionType_${ob.cid}`" name="type" class="clrBr clrP clrSh2 marginTopAuto">
              <template v-for="(shippingType, j) in ob.shippingTypes" :key="j">
                <option :value="shippingType" :selected="ob.type === shippingType">{{ ob.polyT(`editListing.shippingOptions.shippingTypes.${shippingType}`) }}</option>
              </template>
            </select>
          </div>
        </div>
        <div class="flexRow gutterH js-serviceSection" v-show="!ob.type === 'LOCAL_PICKUP'">
          <div class="col3">
            <label class="required">{{ ob.polyT('editListing.shippingOptions.services.nameLabel') }}</label>
          </div>
          <div class="col3">
            <label class="required">{{ ob.polyT('editListing.shippingOptions.services.estimatedDeliveryLabel') }}</label>
          </div>
          <div class="col3">
            <label class="required">{{ ob.polyT('editListing.shippingOptions.services.priceLabel') }}</label>
          </div>
          <div class="col3">
            <label class="required">{{ ob.polyT('editListing.shippingOptions.services.additionalItemPriceLabel') }}</label>
          </div>
        </div>
        <div class="js-servicesWrap js-serviceSection servicesWrap padKids padStack padTop0" v-show="!ob.type === 'LOCAL_PICKUP'"></div>
        <div class="flexRow pad js-serviceSection" v-show="!ob.type === 'LOCAL_PICKUP'">
          <a class="clrBr clrP clrTEm js-btnAddService">{{ ob.polyT('editListing.shippingOptions.services.addService') }}</a>
        </div>
      </form>
    </div>
  </section>
</template>

<script>
import $ from 'jquery';
import '../../../../backbone/utils/lib/selectize';
import loadTemplate from '../../../../backbone/utils/loadTemplate';
import { getTranslatedCountries } from '../../../../backbone/data/countries';
import regions, {
  getTranslatedRegions,
  getIndexedRegions,
} from '../../../../backbone/data/regions';
import ServiceMd from '../../../../backbone/models/listing/Service';
import app from '../../../../backbone/app';
import Service from './Service';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
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
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.model) {
        throw new Error('Please provide a model.');
      }

      const opts = {
        listPosition: 1,
        ...options,
      };

      this.baseInit(opts);
      this.options = opts;

      // get regions
      this.selectCountryData = getTranslatedRegions()
        .map((regionObj) => ({
          id: regionObj.id,
          text: regionObj.name,
          isRegion: true,
        }));

      // now, we'll add in the countries
      const selectCountries = getTranslatedCountries()
        .map((countryObj) => ({
          id: countryObj.dataName,
          text: countryObj.name,
          isRegion: false,
        }));
      this.selectCountryData = this.selectCountryData.concat(selectCountries);

      this.services = this.model.get('services');
      this.serviceViews = [];

      this.listenTo(this.services, 'add', (serviceMd) => {
        const serviceVw = this.createServiceView({
          model: serviceMd,
        });

        this.serviceViews.push(serviceVw);
        this.$servicesWrap.append(serviceVw.render().el);
      });

      this.listenTo(this.services, 'remove', (serviceMd, servicesCl, removeOpts) => {
        const [splicedVw] = this.serviceViews.splice(removeOpts.index, 1);
        splicedVw.remove();
      });
    },

    events () {
      const events = {
        'click .js-removeShippingOption': 'onClickRemoveShippingOption',
        'click .js-btnAddService': 'onClickAddService',
        'click .js-clearAllShipDest': 'onClickClearShipDest',
      };

      events[`change #shipOptionType_${this.model.cid}`] = 'onChangeShippingType';

      return events;
    },

    tagName () {
      return 'section';
    },

    onClickRemoveShippingOption () {
      this.trigger('click-remove', { view: this });
    },

    onClickAddService () {
      this.services
        .push(new ServiceMd());
    },

    onClickClearShipDest () {
      this.$shipDestinationSelect[0]
        .selectize
        .clear();
    },

    onChangeShippingType (e) {
      let method;

      if ($(e.target).val() === 'LOCAL_PICKUP') {
        method = 'addClass';
      } else {
        method = 'removeClass';

        const services = this.model.get('services');

        if (!services.length) services.push(new ServiceMd());
      }

      this.$serviceSection[method]('hide');
    }

  set listPosition (position) {
      if (typeof position !== 'number') {
        throw new Error('Please provide a position as a number');
      }

      const prevPosition = this.options.listPosition;
      const listPosition = this.options.listPosition = position;

      if (listPosition !== prevPosition) {
        this.$headline.text(
          app.polyglot.t('editListing.shippingOptions.optionHeading', { listPosition }),
        );
      }
    }

  get listPosition () {
      return this.options.listPosition;
    },

    getFormData (fields = this.$formFields) {
      const formData = super.getFormData(fields);
      const indexedRegions = getIndexedRegions();

      // Strip out any region elements from shipping destinations
      // drop down. The individual countries will remain.
      formData.regions = formData.regions
        .filter((region) => !indexedRegions[region]);

      return formData;
    }

  // Sets the model based on the current data in the UI.
  setModelData () {
      // set the data for our nested Services views
      this.serviceViews.forEach((serviceVw) => serviceVw.setModelData());
      this.model.set(this.getFormData());
    },

    createServiceView (options) {
      const view = this.createChild(Service, options);

      this.listenTo(view, 'click-remove', (e) => {
        this.services.remove(
          this.services.at(this.serviceViews.indexOf(e.view)),
        );
      });

      return view;
    }

  get $headline () {
      return this._$headline
        || (this._$headline = $('h1'));
    }

  get $shipDestinationDropdown () {
      return this._$shipDestinationDropdown
        || (this._$shipDestinationDropdown = $(`#shipDestinationsDropdown_${this.model.cid}`));
    }

  get $serviceSection () {
      return this._$serviceSection
        || (this._$serviceSection = $('.js-serviceSection'));
    }

  get $formFields () {
      return this._$formFields
        || (this._$formFields = $('select[name], input[name], textarea[name]').filter((index, el) => (
          !$(el).parents('.js-serviceSection').length)));
    }

  /**
   * Returns a list of any regions that are fully represented in the provided
   * countries list.
   */
  representedRegions (countries = []) {
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

    render () {
      super.render();
      loadTemplate('modals/editListing/shippingOption.html', (t) => {
        this.$el.html(t({
          // Since multiple instances of this view will be rendered, any id's should
          // include the cid, so they're unique.
          cid: this.model.cid,
          listPosition: this.options.listPosition,
          shippingTypes: this.model.shippingTypes,
          errors: this.model.validationError || {},
          ...this.model.toJSON(),
        }));

        $(`#shipOptionType_${this.model.cid}`).select2({
          // disables the search box
          minimumResultsForSearch: Infinity,
        });

        this.$shipDestinationSelect = this.getCachedEl(`#shipDestinationsSelect_${this.model.cid}`);
        this.$servicesWrap = $('.js-servicesWrap');

        this.$shipDestinationSelect.selectize({
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
            const { selectize } = this.$shipDestinationSelect[0];

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
            const { selectize } = this.$shipDestinationSelect[0];
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

        this.serviceViews.forEach((serviceVw) => serviceVw.remove());
        this.serviceViews = [];
        const servicesFrag = document.createDocumentFragment();

        this.model.get('services').forEach((serviceMd) => {
          const serviceVw = this.createServiceView({ model: serviceMd });

          this.serviceViews.push(serviceVw);
          serviceVw.render().$el.appendTo(servicesFrag);
        });

        this.$servicesWrap.append(servicesFrag);

        this._$headline = null;
        this._$shipDestinationDropdown = null;
        this._$formFields = null;
        this._$serviceSection = null;
        this._$shipDestinationsSelect = null;
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
