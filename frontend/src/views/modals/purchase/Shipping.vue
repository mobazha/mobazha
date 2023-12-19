<template>
  <div class="shipping">
    <div class="flexVCent">
      <h2 class="h4 required flexExpand">{{ ob.polyT('purchase.shippingTitle') }}</h2>
      <template v-if="ob.userAddresses.length">
        <a class="clrTEm txU tx5b" @click="createNewAddress">{{ ob.polyT('purchase.newAddress') }}</a>
      </template>
    </div>
    <div class="row">
      <template v-if="ob.userAddresses.length">
        <select ref="shippingAddress" @change="changeShippingAddress(val)">
          <option v-for="(a, i) in ob.userAddresses" :key="i" :value="i" :selected="ob.selectedAddressIndex === i">
            {{ getAddress(a) }}
          </option>
        </select>
      </template>

      <template v-else>
        <div class="padGi txCtr">
          <div class="txB row">
            {{ ob.polyT('purchase.noAddresses') }}
          </div>
          <button class="btn clrP clrBr" @click="createNewAddress" >
            {{ ob.polyT('purchase.newAddress') }}
          </button>
        </div>
      </template>
    </div>
    <ShippingOptions :key="model"
      v-if="ob.userAddresses.length"
      :options="{
        validOptions,
        selectedOption,
        getTotalShippingPrice,
      }"
      @shippingOptionSelected="onSelectShippingOption"
    />

  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import app from '../../../../backbone/app';
import ShippingOptionsCol from '../../../../backbone/collections/settings/ShippingOptions';
import ShippingOptions from './ShippingOptions.vue';
import ShippingAddress from '../../../../backbone/models/settings/ShippingAddress';

export default {
  components: {
    ShippingOptions,
  },
  props: {
    options: {
      type: Object,
      default: {
        getTotalShippingPrice: undefined,
      },
	  },
    bb: Function,
  },
  watch: {
    selectedOption() {
      this.$emit('shippingOptionSelected', this.selectedOption)
    },
    selectedAddress() {
      this.updateOptions();
    },
  },
  data () {
    return {
      userAddresses: app.settings.get('shippingAddresses'),
      selectedAddress: app.settings.get('shippingAddresses').at(0) || '',
      selectedOption: {},
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    $(this.$refs.shippingAddress).select2({
      // disables the search box
      minimumResultsForSearch: Infinity,
    });
  },
  computed: {
    ob () {
      const userAddresses = app.settings.get('shippingAddresses');

      const selectedAddressIndex = this.selectedAddress && userAddresses.length ? userAddresses.indexOf(this.selectedAddress) : '';

      return {
        ...this.templateHelpers,
        userAddresses: userAddresses.toJSON(),
        selectedAddressIndex,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (!this.model || !(this.model instanceof ShippingOptionsCol)) {
        throw new Error('Please provide a ShippingOptions model');
      }

      this.validOptions = [];

      const userAddresses = app.settings.get('shippingAddresses');
      this.selectedAddress = userAddresses.at(0) || '';

      this.listenTo(userAddresses, 'update', col => {
        // If all the addresses were deleted, set the selection to blank.
        if (!col.models.length) {
          this.selectedAddress = '';
        } else {
          // If the old selected address doesn't exist any more, select the first address and set the
          // selection to the first valid value.
          this.selectedAddress = userAddresses.get(this.selectedAddress) ? this.selectedAddress : userAddresses.at(0);
        }
      });

      this.updateOptions();
    },

    extractValidOptions(address) {
      // Any time the country is changed, the options valid for that country need to be extracted.
      if (address !== '' && !(address instanceof ShippingAddress)) {
        throw new Error('The address must be blank or an instance of the ShippingAddress model.');
      }

      const validOptions = [];
      const countryCode = address ? address.get('country') : '';

      const extractedOptions = this.model.toJSON().filter(option =>
        option.regions.includes(countryCode) || option.regions.includes('ALL'));

      extractedOptions.forEach(option => {
        if (option.type === 'LOCAL_PICKUP') {
          // local pickup options need a service with a name and price
          option.services[0] = { name: app.polyglot.t('purchase.localPickup'), firstFreight: 0 };
        }
        option.services = _.sortBy(option.services, 'firstFreight');
        option.services.forEach(optionService => {
          validOptions.push({
            ...optionService,
            name: option.name,
            service: optionService.name,
            currency: option.currency,
          });
        });
      });

      return validOptions;
    },

    updateOptions() {
      let address = this.selectedAddress;

      // if the selected address has a new country, extract the valid shipping options.
      if (address && address.get('country') !== this.country) {
        this.validOptions = this.extractValidOptions(address);
      }

      if (this.validOptions.length) {
        // If the previously selected shipping option is no longer valid, select the first valid
        // shipping option. this.selectionOption only has a name and service, as that's the expected
        // data for the server, the validOptions have additional data in them.
        const isSelectedValid = this.selectedOption && this.selectedOption.name &&
          !!this.validOptions.filter(option => option.name === this.selectedOption.name &&
            option.service === this.selectedOption.service).length;

        if (!isSelectedValid) {
          this.selectedOption = {
            name: this.validOptions[0].name,
            service: this.validOptions[0].service,
          };
        }
      } else {
        this.selectedOption = { name: '', service: '' };
      }
    },

    changeShippingAddress (val) {
      this.selectedAddress = app.settings.get('shippingAddresses').at(val);
    },

    onSelectShippingOption(option) {
      this.selectedOption = option
    },

    getAddress (a) {
      const addr = [];
      addr.push(a.name);
      if (a.company) addr.push(a.company);
      if (a.addressLineOne) addr.push(a.addressLineOne);
      if (a.addressLineTwo) addr.push(a.addressLineTwo);
      if (a.city) addr.push(a.city);
      const state = a.state || '';
      const code = a.postalCode || '';
      const stateCode = `${state ? `${state} ` : ''}${code}`;
      if (stateCode) addr.push(stateCode);

      return addr.join(', ');
    },

    createNewAddress () {
      this.$emit('newAddress')
    }
  }
}
</script>
<style lang="scss" scoped></style>
