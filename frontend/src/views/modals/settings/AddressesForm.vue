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
          <input type="text" class="clrBr clrSh2" name="name" id="settingsAddressName" :value="ob.name"
            :placeholder="ob.polyT('settings.addressesTab.placeholderName')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressCompany">{{ ob.polyT('settings.addresses.company') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.company" :errors="ob.errors.company" />
          <input type="text" class="clrBr clrSh2" name="company" id="settingsAddressCompany" :value="ob.company"
            :placeholder="ob.polyT('settings.optional')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressLineOne">{{ ob.polyT('settings.addresses.street') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.addressLineOne" :errors="ob.errors.addressLineOne" />
          <input type="text" class="clrBr clrSh2" name="addressLineOne" id="settingsAddressLineOne"
            :value="ob.addressLineOne" :placeholder="ob.polyT('settings.addressesTab.placeholderAddress')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressLineTwo">{{ ob.polyT('settings.addresses.apt') }}</label>
        </div>
        <div class="col3">
          <FormError v-if="ob.errors.addressLineTwo" :errors="ob.errors.addressLineTwo" />
          <input type="text" class="clrBr clrSh2" name="addressLineTwo" id="settingsAddressLineTwo"
            :value="ob.addressLineTwo" :placeholder="ob.polyT('settings.optional')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressCity">{{ ob.polyT('settings.addresses.city') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.city" :errors="ob.errors.city" />
          <input type="text" class="clrBr clrSh2" name="city" id="settingsAddressCity" :value="ob.city"
            :placeholder="ob.polyT('settings.addressesTab.placeholderCity')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressState">{{ ob.polyT('settings.addresses.state') }}</label>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors.state" :errors="ob.errors.state" />
          <input type="text" class="clrBr clrSh2" name="state" id="settingsAddressState" :value="ob.state"
            :placeholder="ob.polyT('settings.addressesTab.placeholderState')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressPostalCode">{{ ob.polyT('settings.addresses.postalCode') }}</label>
        </div>
        <div class="col3">
          <FormError v-if="ob.errors.postalCode" :errors="ob.errors.postalCode" />
          <input type="text" class="clrBr clrSh2" name="postalCode" id="settingsAddressPostalCode" :value="ob.postalCode"
            :placeholder="ob.polyT('settings.addressesTab.placeholderPostalCode')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressCountry" class="required">{{ ob.polyT('settings.addresses.country') }}</label>
        </div>
        <div class="col6 clrSh2">
          <FormError v-if="ob.errors.country" :errors="ob.errors.country" />
          <select id="settingsAddressCountry" name="country">
            <template v-for="(country, j) in ob.countryList" :key="j">
              <option :value="country.dataName" :selected="country.dataName == ob.country">{{ country.name }}</option>
            </template>
          </select>
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="settingsAddressNotes">{{ ob.polyT('settings.addresses.notes') }}</label>
        </div>
        <div class="col9">
          <FormError v-if="ob.errors.addressNotes" :errors="ob.errors.addressNotes" />
          <textarea rows="6" name="addressNotes" class="clrBr clrSh2" id="settingsAddressNotes"
            :placeholder="ob.polyT('settings.addressesTab.placeholderNotes')">{{ ob.addressNotes }}</textarea>
        </div>
      </div>
    </form>

  </div>
</template>

<script>
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
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
    $('#settingsAddressCountry').select2();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        countryList: this.countryList,
        errors: this.model.validationError || {},
        ...this.model.toJSON(),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit({
        ...options,
      });

      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.countryList = getTranslatedCountries();
    },

    getFormDataEx () {
      const fields = this.$el.querySelectorAll('select[name], input[name], textarea[name]');
      return this.getFormData(fields);
    },
  }
}
</script>
<style lang="scss" scoped></style>
