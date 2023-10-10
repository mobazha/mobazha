<template>
  <div class="settingsAddresses">
    <div class="contentBox padMd clrP clrBr clrSh3">
      <div class="flexHCent">
        <h2 class="h3 clrT">{{ ob.polyT('settings.addressesTab.sectionName') }}</h2>
      </div>
      <hr class="clrBr" />

      <div class="tabFormWrapper js-addressesWrap clrP">
        <div class="settingsTabFormWrapperInner">
          <div class="js-listContainer"></div>
          <div class="js-formContainer"></div>
        </div>
      </div>

      <div class="flexHRight">
        <ProcessingButton className="btn clrP clrBAttGrad clrBrDec1 clrTOnEmph" @click="saveNewAddress"
          :btnText="ob.polyT('settings.btnAddAddress')" />
      </div>
    </div>
  </div>
</template>

<script>
import app from '../../../../backbone/app';
import { openSimpleMessage } from '../SimpleMessage';
import AddressesForm from './AddressesForm';
import AddressesList from './AddressesList';
import ShippingAddress from '../../../../backbone/models/settings/ShippingAddress';


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
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: {},
        ...this._settings,
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
      this.listenTo(app.settings, 'someChange', (md, opts) =>
        this.settings.set(opts.setAttrs));

      // Sync the global settings model with any changes we save via our clone.
      this.listenTo(this.settings, 'sync', (md, resp, opts) =>
        app.settings.set(this.settings.toJSON(opts.attrs)));

      this.addressForm = this.createChild(AddressesForm, { model: new ShippingAddress() });

      this.addressList = this.createChild(AddressesList,
        { collection: this.settings.get('shippingAddresses') });
      this.listenTo(this.addressList, 'deleteAddress', this.onDeleteAddress);
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
          .always(() => setTimeout(() => statusMessage.remove(), 3000));
      }
    },

    saveNewAddress () {
      const model = this.addressForm.model;
      const formData = this.addressForm.getFormData();

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
          this.$btnAddAddress.addClass('processing');
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

            this.addressForm.model = new ShippingAddress();
            this.addressForm.render();
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
            this.$btnAddAddress.removeClass('processing');
            setTimeout(() => statusMessage.remove(), 3000);
          });
        }
      }

      // render so errors are shown / cleared
      this.addressForm.render();

      if (this.settings.validationError || model.validationError) {
        const $firstFormErr = $('.js-formContainer .errorList:first');

        if ($firstFormErr.length) {
          $firstFormErr[0].scrollIntoViewIfNeeded();
        } else {
          this.$emit('unrecognizedModelError', this, [this.settings]);
        }
      }
    },

    get $btnAddAddress () {
      return this._$btnAddAddress || $('.js-addAddress');
    },

    render () {
      $('.js-formContainer').html(
        this.addressForm.render().el
      );

      $('.js-listContainer').html(
        this.addressList.render().el
      );

      this._$btnAddAddress = null;

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
