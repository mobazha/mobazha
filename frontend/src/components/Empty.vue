<template>
  <div class="empty" v-if="emptyInfo">
    <img class="empty-icon" :src="emptyInfo.icon" />
    <div class="empty-name">{{ emptyInfo.name }}</div>
    <div class="empty-desc">{{ emptyInfo.desc }}</div>
    <div class="btn-primary empty-btn" @click="navPage">{{ emptyInfo.btn }}</div>
  </div>
</template>
<script setup>
defineProps({
  emptyInfo: {
    type: Object,
    default: '',
  },
});
const router = useRouter();
function navPage() {
  router.push('/store');
}
</script>
<style lang="scss" scoped>
.empty {
  width: 100%;
  height: 500px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  background: #fff;
  border: 1px solid #e0e0e0;
  box-sizing: border-box;
  &-icon {
    width: 161px;
    height: 123px;
  }
  &-name {
    color: #000;
    font-size: 30px;
    line-height: normal;
    margin: 26px 0 20px 0;
  }
  &-desc {
    color: #000;
    font-size: 14px;
    line-height: normal;
    margin-bottom: 80px;
  }
  &-btn {
    padding: 0 30px;
  }
}
</style>
