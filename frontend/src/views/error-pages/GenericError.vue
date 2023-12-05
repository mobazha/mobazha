<template>
  <div>
    <div class="tx5">
      <h1 class="txCtr">{{ ob.polyT('errorPage.genericErrorTitle') }}</h1>
      <p v-if="content" class="txCtr" v-html="content"></p>
    </div>

  </div>
</template>

<script>

export default {
  props: {
    content: {
      type: String,
      default: '',
    },
  },
}
</script>
<style lang="scss" scoped></style>
