<template>
  <div class="modal modalTop modalScrollPage modalMedium">
    <BaseModal>
      <template v-slot:component>
        <div class="topControls flex"></div>
        <div class="contentBox padMd clrP clrBr clrSh3">
          <h1>{{ ob.polyT('editListing.unsupportedCurrencyDialog.title') }}</h1>
          <p>{{ ob.polyT('editListing.unsupportedCurrencyDialog.body', { cur: ob.unsupportedCurrency }) }}</p>
          <div class="flexCent row">
            <div class="col6">
              <Select2 class="clrBr clrP js-currencyList" v-model="newCur">
                <template v-for="(currency, j) in ob.curList" :key="j">
                  <option :value="currency.code" :selected="currency.code === newCur">{{ `${currency.code} - ${ob.polyT(`currencies.${currency.code}`)}` }}</option>
                </template>
              </Select2>
            </div>
          </div>
          <div class="flexCent">
            <button class="btn clrP clrBr" @click="onClickOkCurrencySet">{{ ob.polyT('editListing.unsupportedCurrencyDialog.btnOk') }}</button>
          </div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../../backbone/app';
import { getCurrenciesSortedByCode } from '../../../../backbone/data/currencies';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      newCur: app.settings.get('localCurrency'),
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      this.curList = this.curList || getCurrenciesSortedByCode();

      return {
        ...this.templateHelpers,
        unsupportedCurrency: this.unsupportedCurrency,
        curList: this.curList,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        removeOnClose: true,
        dismissOnOverlayClick: false,
        dismissOnEscPress: false,
        showCloseButton: false,
        ...options,
      };

      if (typeof opts.unsupportedCurrency !== 'string') {
        throw new Error('Please provide the unsupported currency code as a string.');
      }

      this.baseInit(opts);
    },

    onClickOkCurrencySet () {
      this.close();
    },

    getCurrency () {
      return this.newCur;
    },
  }
}
</script>
<style lang="scss" scoped></style>
