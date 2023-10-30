<template>
  <div class="socialAccount">
    <div>
      <FormError v-if="ob.errors['type']" :errors="ob.errors['type']" />
      <FormError v-if="ob.errors['username']" :errors="ob.errors['username']" />
      <FormError v-if="ob.errors['duplicate']" :errors="ob.errors['duplicate']" />
    </div>
    <div class="posR">
      <div class="flexRow gutterH">
        <div class="col6">
          <input type="text" class="clrBr clrSh2" name="type" :value="ob.type"
            :placeholder="ob.polyT('settings.socialAccounts.accountPlaceholder')">
        </div>
        <div class="col6">
          <input type="text" class="clrBr clrSh2" name="username" :value="ob.username"
            :placeholder="ob.polyT('settings.socialAccounts.usernamePlaceholder')">
        </div>
      </div>
      <div class="deleteAccountWrapper margL">
        <button class="iconBtn form ion-trash-b clrP clrBr " @click="onClickRemove"></button>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';

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
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        errors: this.model.validationError || {},
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

    onClickRemove () {
      this.$emit('remove-click', { view: this });
    },

    getFormDataEx () {
      const fields = this.$el.querySelectorAll('input[name]');
      return this.getFormData(fields);
    },

    get firstBlankField () {
      const formData = this.getFormDataEx();
      return _.findKey(formData, val => !val);
    },

    // Sets the model based on the current data in the UI.
    setModelData () {
      this.model.set(this.getFormDataEx());
    },

  }
}
</script>
<style lang="scss" scoped></style>
