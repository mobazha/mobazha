<template>
  <div class="modal loadingModal clrP clrT">
    <BaseModal>
      <template v-slot:component>
        <div class="flexCent">
          <SpinnerSVG className="spinnerLg" />
        </div>
        <div class="content js-content" v-html="contentView"></div>
      </template>
    </BaseModal>
  </div>
</template>

<script>


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      contentView: '',
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    }
  },
  methods: {
    open (contentView) {
      this.contentView = contentView;
    },
  }
}
</script>
<style lang="scss" scoped></style>
