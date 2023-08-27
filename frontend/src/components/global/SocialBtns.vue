<template>
  <div :class="ob.stripClasses">
    <a :class="`btn ${ob.btnClasses} js-messageBtn`">{{ ob.polyT('userPage.message') }}</a>
    <ProcessingButton
      :className="`btn ${ob.btnClasses} ${ob.isFollowing ? 'processing' : ''} js-followUnfollowBtn`"
      :btnText="ob.following ? ob.polyT('follow.unfollowBtn') : ob.polyT('follow.followBtn')" />
    <div class="js-blockBtnContainer"></div>
  </div>

</template>

<script setup>
const props = defineProps({
  phase: String,
})

</script>
<style lang="scss" scoped>
</style>