<template>
  <div class="listFetcher">
    <template v-if="ob.isFetching">
      <div class="loadingSpinnerWrap">
        <SpinnerSVG className="spinnerMd" />
      </div>
    </template>

    <template v-else-if="ob.fetchFailed">
      <p>{{ ob.polyT('notifications.errorHeading') }}</p>
      <p v-if="ob.fetchError">{{ ob.fetchError }}</p>
      <button class="btn normalBtn clrP clrBr " @click="onClickRetry">{{ ob.polyT('notifications.btnRetry') }}</button>
    </template>

    <template v-else-if="ob.noResults">
      <p>{{ ob.polyT('notifications.noResults') }}</p>
    </template>

  </div>
</template>

<script>
/*
  ListFetcher is a bit of a misnomer. This view doesn't really fetch the notifications, it just
  displays the status of the fetch (i.e. a spinner during the fetch and an error message and
  retry button if it fails).
*/

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
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        initialState: {
          isFetching: false,
          noResults: false,
          fetchError: '',
          ...options.initialState || {},
        },
        ...options,
      };

      this.baseInit(opts);
    },

    onClickRetry () {
      this.$emit('retry-click');
    },
  }
}
</script>
<style lang="scss" scoped></style>
