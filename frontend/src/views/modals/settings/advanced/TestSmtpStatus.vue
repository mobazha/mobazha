<template>
  <div class="testSmtpStatus">
    <template v-if="ob.msg">
      <template v-if="ob.success">
        <p class="successMsg"><span class="successIcon ion-ios-checkmark-empty padSm"></span>{{ ob.msg }}</p>
      </template>

      <template v-else-if="ob.msg">
        <p class="clrTErr"><span class="ion-alert-circled padSm"></span>{{ ob.msg }}</p>
      </template>
    </template>

  </div>
</template>

<script>
export default {
  props: {
    options: {
      type: Object,
      default: {
        success: true,
        msg: '',
      },
    },
  },
  data () {
    return {
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
        success: true,
        msg: '',
        ...this.options,
      };
    }
  },
  methods: {

  }
}
</script>
<style lang="scss" scoped></style>
