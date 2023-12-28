<template>
  <div class="listFetcher">
    <template v-if="fetchState.isFetching">
      <div class="loadingSpinnerWrap">
        <SpinnerSVG className="spinnerMd" />
      </div>
    </template>

    <template v-else-if="fetchState.fetchFailed">
      <p>{{ ob.polyT('notifications.errorHeading') }}</p>
      <p v-if="fetchState.fetchError">{{ fetchState.fetchError }}</p>
      <button class="btn normalBtn clrP clrBr " @click="onClickRetry">{{ ob.polyT('notifications.btnRetry') }}</button>
    </template>

    <template v-else-if="fetchState.noResults">
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
    fetchState: {
      type: Object,
      default: {
        isFetching: false,
        fetchFailed: false,
        noResults: false,
        fetchError: '',
      },
    },
    bb: Function,
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();
  },
  mounted () {
  },
  computed: {

  },
  methods: {
    onClickRetry () {
      this.$emit('retry-click');
    },
  }
}
</script>
<style lang="scss" scoped></style>
