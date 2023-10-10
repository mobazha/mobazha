<template>
  <div class="variant flexRow gutterH">
    <div class="col6 simpleFlexCol">
      <FormError v-if="ob.errors['name']" :errors="ob.errors['name']" />
      <input type="text" class="clrBr clrP clrSh2 variantNameInput js-variantNameInput" name="name" :value="ob.name"
        :placeholder="ob.polyT('editListing.variants.titlePlaceholder')" :maxlength="ob.max.nameLength">
    </div>
    <div class="col6 simpleFlexCol">
      <FormError v-if="variantsErrs.length" :errors="variantsErrs" />
      <div class="flexRow marginTopAuto">
        <select multiple name="variants" class="clrBr clrP clrSh2 hideDropDown flexExpand"
          :placeholder="ob.polyT('editListing.variants.choicesPlaceholder')"></select>
        <a class="iconBtn clrBr clrP clrSh2 margLSm toolTipNoWrap  btnRemoveVariant" @click="onClickRemove"
          :data-tip="ob.polyT('editListing.variants.toolTip.delete')">
          <i class="ion-trash-b"></i>
        </a>
      </div>
    </div>

  </div>
</template>

<script>
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
      const errors = {
        ...(this.model.validationError || {}),
        ...(this.options.errors || {}),
      };

      return {
        ...this.templateHelpers,
        ...this._model,
        max: this.model.max,
        errors,
      };
    },
    variantsErrs () {
      const ob = this.ob;

      let variantsErrs = [];

      Object.keys(ob.errors).forEach(errKey => {
        if (errKey.startsWith('variants[') && errKey.endsWith('].name')) {
          variantsErrs = variantsErrs.concat(ob.errors[errKey]);
        }
      });

      if (ob.errors['variants']) {
        variantsErrs = variantsErrs.concat(ob.errors['variants']);
      }

      return variantsErrs;
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a VariantOption model.');
      }

      // any parent level errors can be passed in options.errors, e.g.
      // options.errors = {
      //   <field-name>: ['err1', 'err2', 'err3']
      // }

      this.baseInit(options);
    },

    onClickRemove () {
      this.$emit('removeClick', { view: this });
    },

    getFormDataEx (fields = this.$formFields) {
      const formData = this.getFormData(fields);

      // Post process the vairants to seperate the clientID from the actual value.
      formData.variants = formData.variants.map((v) => {
        if (v.includes('<===>')) {
          const split = v.split('<===>');
          return {
            _clientID: split[0],
            name: split[1],
          };
        }

        return { name: v };
      });

      return formData;
    },

    // Sets the model based on the current data in the UI.
    setModelData () {
      const formData = this.getFormDataEx();
      this.model.set(formData);
    },

    get $formFields () {
      return this._$formFields
        || (this._$formFields = $('select[name], input[name], textarea[name]'));
    },

    render () {
      this.$el.toggleClass('hasError', !!Object.keys(errors).length);

      this._$formFields = null;

      const variantItems = [];
      const variantOptions = [];

      this.model.get('variants').toJSON()
        .forEach((variant) => {
          const value = `${variant._clientID}<===>${variant.name}`;
          variantOptions.push({ ...variant, value });
          variantItems.push(value);
        });

      $('select[name=variants]').selectize({
        persist: false,
        valueField: 'value',
        options: variantOptions,
        items: variantItems,
        create: (input) => ({
          name: input,
          value: input,
        }),
        render: {
          option: (data) => `<div>${data.name}</div>`,
          item: (data) => `<div>${data.name}</div>`,
        },
      }).on('change', () => this.trigger('choiceChange', { view: this }));

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
