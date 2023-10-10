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
      default: {},
    },
  },
  data () {
    return {
      _state: {
        isFetching: false,
        success: true,
        msg: '',
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
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        initialState: {
          isFetching: false,
          success: true,
          msg: '',
          ...options.initialState || {},
        },
        ...options,
      };

      this.baseInit(opts);
    },

  }
}
</script>
<style lang="scss" scoped></style>
