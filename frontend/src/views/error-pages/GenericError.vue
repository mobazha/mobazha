<template>
  <div>
    <div class="tx5">
      <h1 class="txCtr">{{ ob.polyT('errorPage.genericErrorTitle') }}</h1>
      <template v-if="options.content">
        <p class="txCtr" v-html="options.content"></p>
      </template>
    </div>

  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {
        content: '',
      },
    },
  },
}
</script>
<style lang="scss" scoped></style>
