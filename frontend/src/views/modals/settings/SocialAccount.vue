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
          <input type="text" class="clrBr clrSh2" ref="type" v-model="formData.type" :placeholder="ob.polyT('settings.socialAccounts.accountPlaceholder')">
        </div>
        <div class="col6">
          <input type="text" class="clrBr clrSh2" ref="username" v-model="formData.username" :placeholder="ob.polyT('settings.socialAccounts.usernamePlaceholder')">
        </div>
      </div>
      <div class="deleteAccountWrapper margL">
        <button class="iconBtn form ion-trash-b clrP clrBr " @click.stop="onClickRemove"></button>
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
      formData: {
        type: '',
        username: '',
      }
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
        errors: this.model.validationError || {},
      };
    },
    firstBlankField() {
      if (!this.formData.type) {
        return 'type';
      } else if (!this.formData.username) {
        return 'username';
      }
      return '';
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.baseInit(options);
      this.formData = {
        type: this.model.get('type'),
        username: this.model.get('username'),
      }
    },

    setFocus(fieldName) {
      if (this.$refs[fieldName]) {
        this.$refs[fieldName].focus();
      }
    },

    onClickRemove () {
      this.$emit('remove-click');
    },

    // Sets the model based on the current data in the UI.
    setModelData () {
      this.model.set(this.formData);
    },
  }
}
</script>
<style lang="scss" scoped></style>
