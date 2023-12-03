<template>
  <div class="settingsAddresses">
    <div class="contentBox padMd clrP clrBr clrSh3">
      <div class="flexHCent">
        <h2 class="h3 clrT">{{ ob.polyT('settings.addressesTab.sectionName') }}</h2>
      </div>
      <hr class="clrBr" />

      <div class="tabFormWrapper js-addressesWrap clrP">
        <div class="settingsTabFormWrapperInner">
          <div class="js-listContainer">
            <AddressesList
              :key="addressesListKey"
              :bb="() => {
                return {
                  collection: settings.get('shippingAddresses')
                };
              }"
              @deleteAddress="onDeleteAddress"
            />
          </div>
          <div class="js-formContainer">
            <AddressesForm ref="addressesForm"
              :bb="() => {
                return {
                  model: addressesFormModel
                };
              }"
            />
          </div>
        </div>
      </div>

      <div class="flexHRight">
        <ProcessingButton :className="`btn clrP clrBAttGrad clrBrDec1 clrTOnEmph ${isAdding ? 'processing' : ''}`" @click="saveNewAddress"
          :btnText="ob.polyT('settings.btnAddAddress')" />
      </div>
    </div>
  </div>
</template>

<script>
import app from '../../../../backbone/app';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import ShippingAddress from '../../../../backbone/models/settings/ShippingAddress';

import AddressesForm from './AddressesForm.vue';
import AddressesList from './AddressesList.vue';


export default {
  components: {
    AddressesForm,
    AddressesList,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      isAdding: false,

      addressesFormModel: new ShippingAddress(),

      addressesListKey: 0,
    };
  },
  created () {
    this.initEventChain();
  
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: {},
        ...this.settings.toJSON(),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit({
        ...options,
      });

      this.settings = app.settings.clone();

      // Sync our clone with any changes made to the global settings model.
      this.listenTo(app.settings, 'someChange', (md, opts) => {
        this.settings.set(opts.setAttrs)
      });

      // Sync the global settings model with any changes we save via our clone.
      this.listenTo(this.settings, 'sync', (md, resp, opts) =>
        app.settings.set(this.settings.toJSON(opts.attrs)));
    },

    onDeleteAddress (address) {
      const shippingAddresses = this.settings.get('shippingAddresses');
      const removeIndex = shippingAddresses.indexOf(address);

      this.settings.set({}, { validate: true });

      if (!this.settings.validationError) {
        shippingAddresses.remove(address);
      } else {
        this.$emit('unrecognizedModelError', this, [this.settings]);
        return;
      }

      const save = this.settings.save({ shippingAddresses: shippingAddresses.toJSON() }, {
        attrs: { shippingAddresses: shippingAddresses.toJSON() },
        type: 'PUT',
      });

      if (save) {
        const truncatedName = address.get('name').slice(0, 30);

        const msg = {
          msg: app.polyglot.t('settings.addressesTab.statusDeletingAddress',
            { name: `<em>${truncatedName}</em>` }),
          type: 'message',
        };

        const statusMessage = app.statusBar.pushMessage({
          ...msg,
          duration: 9999999999999999,
        });

        save.done(() => {
          statusMessage.update({
            msg: app.polyglot.t('settings.addressesTab.statusDeleteAddressComplete',
              { name: `<em>${truncatedName}</em>` }),
            type: 'confirmed',
          });
        })
          .fail((...args) => {
            // put the address that failed to remove back in
            shippingAddresses.add(address, { at: removeIndex });

            const errMsg = args[0] && args[0].responseJSON &&
              args[0].responseJSON.reason || '';

            openSimpleMessage(
              app.polyglot.t('settings.addressesTab.deleteAddressErrorAlertTitle',
                { name: `<em>${truncatedName}</em>` }),
              errMsg
            );

            statusMessage.update({
              msg: app.polyglot.t('settings.addressesTab.statusDeleteAddressFailed',
                { name: `<em>${truncatedName}</em>` }),
              type: 'warning',
            });
          })
          .always(() => {
            this.addressesListKey += 1;
            setTimeout(() => statusMessage.remove(), 3000)}
          );
      }
    },

    saveNewAddress () {
      const model = this.addressesFormModel;
      const formData = this.$refs.addressesForm.getFormData();

      model.set(formData);
      model.set(formData, { validate: true });
      this.settings.set({}, { validate: true });

      if (!this.settings.validationError && !model.validationError) {
        const shippingAddresses = this.settings.get('shippingAddresses');

        shippingAddresses.push(model);

        const save = this.settings.save({ shippingAddresses: shippingAddresses.toJSON() }, {
          attrs: { shippingAddresses: shippingAddresses.toJSON() },
          type: 'PUT',
        });

        if (save) {
          this.isAdding = true;
          const truncatedName = model.get('name').slice(0, 30);

          const msg = {
            msg: app.polyglot.t('settings.addressesTab.statusAddingAddress',
              { name: `<em>${truncatedName}</em>` }),
            type: 'message',
          };

          const statusMessage = app.statusBar.pushMessage({
            ...msg,
            duration: 9999999999999999,
          });

          save.done(() => {
            statusMessage.update({
              msg: app.polyglot.t('settings.addressesTab.statusAddAddressComplete',
                { name: `<em>${truncatedName}</em>` }),
              type: 'confirmed',
            });

            this.addressesFormModel = new ShippingAddress();
          }).fail((...args) => {
            // remove the address that failed to add
            // todo: can't remove by passing in model instance from above because of some
            // weirdness with _clientID... investigate.
            const modelToRemove = shippingAddresses.findWhere({ name: model.get('name') });
            if (modelToRemove) shippingAddresses.remove(modelToRemove);

            const errMsg = args[0] && args[0].responseJSON && args[0].responseJSON.reason || '';

            openSimpleMessage(
              app.polyglot.t('settings.addressesTab.addAddressErrorAlertTitle',
                { name: `<em>${truncatedName}</em>` }),
              errMsg
            );

            statusMessage.update({
              msg: app.polyglot.t('settings.addressesTab.statusAddAddressFailed',
                { name: `<em>${truncatedName}</em>` }),
              type: 'warning',
            });
          }).always(() => {
            this.isAdding = false;
            this.addressesListKey += 1;

            setTimeout(() => statusMessage.remove(), 3000);
          });
        }
      }

      if (this.settings.validationError || model.validationError) {
        const $firstFormErr = $('.js-formContainer .errorList:first');

        if ($firstFormErr.length) {
          $firstFormErr[0].scrollIntoViewIfNeeded();
        } else {
          this.$emit('unrecognizedModelError', this, [this.settings]);
        }
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
