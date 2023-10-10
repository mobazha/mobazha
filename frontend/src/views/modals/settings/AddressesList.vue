<template>
  <div>
    <div class="addresses tx5 gutterV">
      <template v-for="(addressRow, index1) in ob.addresses">
        <div class="flexRow gutterH">
          <template v-for="(address, index2) in addressRow">
            <div class="col6">
              <div class="addressBox border clrP clrBr">
                <span class="txB">{{ `${address.name}${address.company ? '<br />' : ''}` }}</span>
                {{ `${address.company}${address.addressLineOne ? '<br />' : ''}` }}
                {{ `${address.addressLineOne}${address.addressLineTwo ? ` ${address.addressLineTwo}` : ''}${address.city ? '<br />' : ''}` }}
                {{ `${address.city}${address.state ? `, ${address.state}` : ''}${address.postalCode ? ` ${address.postalCode}` : ''}${address.country ? '<br />' : ''}` }}
                {{ `${ob.polyT(`countries.${address.country}`)}${address.addressNotes ? '<br />' : ''}` }}
                <p v-if="address.addressNotes" class="notes">{{ address.addressNotes }}</p>
                <a class="btn clrP clrBr clrSh2 " @click="onClickDelete(index1 * 2 + index2)">{{ ob.polyT('settings.addressesTab.btnDelete') }}</a>
              </div>
            </div>
          </template>
        </div>
      </template>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import { splitIntoRows } from '../../../../backbone/utils';

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
      return {
        ...this.templateHelpers,
        errors,
        addresses: splitIntoRows(this._collection, 2),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit({
        ...options,
      });

      if (!this.collection) {
        throw new Error('Please provide a collection.');
      }
    },

    onClickDelete (index) {
      this.$emit('deleteAddress', this.collection.at(index));
    },
  }
}
</script>
<style lang="scss" scoped></style>
