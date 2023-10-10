<template>
  <div class="flexRow gutterH">
    <div class="col3 simpleFlexCol">
      <FormError v-if="ob.errors['name']" :errors="ob.errors['name']" />
      <input type="text" class="clrBr clrP clrSh2 marginTopAuto" name="name" :value="ob.name"
        :placeholder="ob.polyT('editListing.shippingOptions.services.namePlaceholder')">
    </div>
    <div class="col3 simpleFlexCol">
      <FormError v-if="ob.errors['estimatedDelivery']" :errors="ob.errors['estimatedDelivery']" />
      <input type="text" class="clrBr clrP clrSh2 marginTopAuto" name="estimatedDelivery" :value="ob.estimatedDelivery"
        :placeholder="ob.polyT('editListing.shippingOptions.services.estimatedDeliveryPlaceholder')">
    </div>
    <div class="col3 simpleFlexCol">
      <FormError v-if="ob.errors['price']" :errors="ob.errors['price']" />
      <input type="text" class="clrBr clrP clrSh2 marginTopAuto js-price" name="price"
        :value="ob.number.toStandardNotation(ob.price)"
        :placeholder="ob.polyT('editListing.shippingOptions.services.pricePlaceholder')" data-var-type="bignumber">
    </div>
    <div class="col3 simpleFlexCol">
      <FormError v-if="ob.errors['additionalItemPrice']" :errors="ob.errors['additionalItemPrice']" />
      <div class="flexRow marginTopAuto">
        <input type="text" class="clrBr clrP clrSh2 marginTopAuto js-price" name="additionalItemPrice"
          :value="ob.number.toStandardNotation(ob.additionalItemPrice)"
          :placeholder="ob.polyT('editListing.shippingOptions.services.pricePlaceholder')" data-var-type="bignumber">
        <a class="iconBtn clrBr clrP margLSm toolTipNoWrap  btnRemoveService" @click="onClickRemoveService"
          :data-tip="ob.polyT('editListing.shippingOptions.toolTip.delete')">
          <i class="ion-trash-b"></i>
        </a>
      </div>
    </div>

  </div>
</template>

<script>
import loadTemplate from '../../../../backbone/utils/loadTemplate';

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
        // Since multiple instances of this view will be rendered, any id's should
        // include the cid, so they're unique.
        cid: this.model.cid,
        errors: this.model.validationError || {},
        ...this._model,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.baseInit(options);
    },

    onClickRemoveService () {
      this.trigger('click-remove', { view: this });
    },

    // Sets the model based on the current data in the UI.
    setModelData () {
      this.model.set(this.getFormData(this.$formFields));
    },

  get $formFields () {
      return this._$formFields ||
        (this._$formFields =
          $('select[name], input[name], textarea[name]'));
    },

    render () {
      this._$formFields = null;

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
