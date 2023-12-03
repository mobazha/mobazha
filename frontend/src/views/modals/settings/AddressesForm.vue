<template>
  <div class="settingsAddressesForm">
    <h2 class="h4 clrT">{{ ob.polyT('settings.addresses.sectionName') }}</h2>
    <form class="padKids padStack contentBox pad rowMd clrP border clrBr js-addAddressForm">
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressName" class="required">{{ ob.polyT('settings.addresses.name') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.name" :errors="ob.errors.name" />
          <input type="text" class="clrBr clrSh2" v-model="formData.name" id="settingsAddressName"
            :placeholder="ob.polyT('settings.addressesTab.placeholderName')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressCompany">{{ ob.polyT('settings.addresses.company') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.company" :errors="ob.errors.company" />
          <input type="text" class="clrBr clrSh2" v-model="formData.company" id="settingsAddressCompany"
            :placeholder="ob.polyT('settings.optional')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressLineOne">{{ ob.polyT('settings.addresses.street') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.addressLineOne" :errors="ob.errors.addressLineOne" />
          <input type="text" class="clrBr clrSh2" v-model="formData.addressLineOne" id="settingsAddressLineOne"
            :placeholder="ob.polyT('settings.addressesTab.placeholderAddress')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressLineTwo">{{ ob.polyT('settings.addresses.apt') }}</label>
        </div>
        <div class="col3">
          <FormError v-if="ob.errors.addressLineTwo" :errors="ob.errors.addressLineTwo" />
          <input type="text" class="clrBr clrSh2" v-model="formData.addressLineTwo" id="settingsAddressLineTwo"
            :placeholder="ob.polyT('settings.optional')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressCity">{{ ob.polyT('settings.addresses.city') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.city" :errors="ob.errors.city" />
          <input type="text" class="clrBr clrSh2" v-model="formData.city" id="settingsAddressCity"
            :placeholder="ob.polyT('settings.addressesTab.placeholderCity')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressState">{{ ob.polyT('settings.addresses.state') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.state" :errors="ob.errors.state" />
          <input type="text" class="clrBr clrSh2" v-model="formData.state" id="settingsAddressState"
            :placeholder="ob.polyT('settings.addressesTab.placeholderState')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressPostalCode">{{ ob.polyT('settings.addresses.postalCode') }}</label>
        </div>
        <div class="col3">
          <FormError v-if="ob.errors.postalCode" :errors="ob.errors.postalCode" />
          <input type="text" class="clrBr clrSh2" v-model="formData.postalCode" id="settingsAddressPostalCode"
            :placeholder="ob.polyT('settings.addressesTab.placeholderPostalCode')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressCountry" class="required">{{ ob.polyT('settings.addresses.country') }}</label>
        </div>
        <div class="col6 clrSh2">
          <FormError v-if="ob.errors.country" :errors="ob.errors.country" />
          <Select2 id="settingsAddressCountry" v-model="formData.country">
            <template v-for="(country, j) in countryList" :key="j">
              <option :value="country.dataName" :selected="country.dataName == formData.country">{{ country.name }}</option>
            </template>
          </Select2>
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressNotes">{{ ob.polyT('settings.addresses.notes') }}</label>
        </div>
        <div class="col9">
          <FormError v-if="ob.errors.addressNotes" :errors="ob.errors.addressNotes" />
          <textarea rows="6" v-model="formData.addressNotes" class="clrBr clrSh2" id="settingsAddressNotes"
            :placeholder="ob.polyT('settings.addressesTab.placeholderNotes')"></textarea>
        </div>
      </div>
    </form>

  </div>
</template>

<script>
import _ from 'underscore';
import { getTranslatedCountries } from '../../../../backbone/data/countries';

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
      countryList: undefined,

      formData: {
        name: '',
        company: '',
        addressLineOne: '',
        addressLineTwo: '',
        city: '',
        state: '',
        postalCode: '',
        country: '',
        addressNotes: '',
      }
    };
  },
  created () {
    this.loadData();
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: this.model.validationError || {},
      };
    }
  },
  methods: {
    loadData () {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.countryList = getTranslatedCountries();

      this.formData = _.pick(this.model.toJSON(), _.keys(this.formData));
    },

    getFormData () {
      return this.formData;
    },
  }
}
</script>
<style lang="scss" scoped></style>
