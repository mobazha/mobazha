<template>
  <div>
    <div class="tx5">
      <h1 class="txCtr">{{ ob.polyT('errorPage.userNotFoundTitle') }}</h1>
      <template v-if="options.user">
        <p class="txCtr">{{ ob.polyT('errorPage.userNotFoundMsg', { user: options.user }) }}</p>
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
        user: '',
      },
    },
  },
}
</script>
<style lang="scss" scoped></style>
